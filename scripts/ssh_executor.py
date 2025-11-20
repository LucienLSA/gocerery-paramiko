#!/usr/bin/env python3
"""
Executor script that connects to target servers via a bastion host using Paramiko.
The script expects JSON payloads passed through CLI arguments so that the Go
service can invoke it securely.
"""

import argparse
import json
import logging
import os
import queue
import sys
import threading
from typing import Any, Dict, List

import paramiko

# 配置日志
def setup_logging(log_level: str = "INFO", log_file: str = None):
    """设置日志配置"""
    level = getattr(logging, log_level.upper(), logging.INFO)
    
    handlers = [logging.StreamHandler(sys.stderr)]  # 默认输出到 stderr（会被 Go 捕获）
    
    if log_file:
        # 确保日志目录存在
        log_dir = os.path.dirname(log_file)
        if log_dir and not os.path.exists(log_dir):
            os.makedirs(log_dir, exist_ok=True)
        handlers.append(logging.FileHandler(log_file))
    
    logging.basicConfig(
        level=level,
        format='%(asctime)s [%(levelname)s] %(name)s: %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S',
        handlers=handlers
    )
    
    # 设置 Paramiko 日志级别（可选，用于调试）
    paramiko_logger = logging.getLogger("paramiko")
    paramiko_logger.setLevel(logging.WARNING)  # 默认只显示警告和错误
    
    return logging.getLogger(__name__)

# 全局日志对象（在 main 中初始化）
logger = None


def build_result(target: Dict[str, Any]) -> Dict[str, Any]:
    return {
        "name": target.get("name"),
        "host": target.get("host"),
        "success": True,
        "stdout": "",
        "stderr": "",
        "exit_code": 0,
        "error": "",
    }


def connect_via_bastion(bastion: Dict[str, Any], target: Dict[str, Any], timeout: int):
    if logger:
        logger.info(f"Connecting to bastion {bastion['host']}:{bastion.get('port', 22)}")
    
    bastion_client = paramiko.SSHClient()
    bastion_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    bastion_client.connect(
        hostname=bastion["host"],
        port=bastion.get("port", 22),
        username=bastion["user"],
        password=bastion["password"],
        timeout=timeout,
    )

    transport = bastion_client.get_transport()
    if transport is None:
        if logger:
            logger.error("Unable to obtain bastion transport")
        raise RuntimeError("unable to obtain bastion transport")

    dest_addr = (target["host"], target.get("port", 22))
    local_addr = ("127.0.0.1", 0)
    channel = transport.open_channel("direct-tcpip", dest_addr, local_addr)

    if logger:
        logger.info(f"Opening channel to target {target['host']}:{target.get('port', 22)}")
    
    target_client = paramiko.SSHClient()
    target_client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    target_client.connect(
        hostname=target["host"],
        port=target.get("port", 22),
        username=target["user"],
        password=target["password"],
        sock=channel,
        timeout=timeout,
    )

    if logger:
        logger.info(f"Successfully connected to target {target['host']}")
    
    return bastion_client, target_client


def run_commands(
    bastion: Dict[str, Any],
    target: Dict[str, Any],
    commands: List[str],
    timeout: int,
) -> Dict[str, Any]:
    result = build_result(target)
    bastion_client = None
    target_client = None

    target_name = target.get("name", target.get("host", "unknown"))
    
    try:
        if logger:
            logger.info(f"Starting command execution on {target_name} ({target.get('host')})")
        
        bastion_client, target_client = connect_via_bastion(bastion, target, timeout)
        for i, command in enumerate(commands, 1):
            if logger:
                logger.info(f"Executing command {i}/{len(commands)} on {target_name}: {command}")
            
            stdin, stdout, stderr = target_client.exec_command(command, timeout=timeout)
            out = stdout.read().decode(errors="ignore")
            err = stderr.read().decode(errors="ignore")
            result["stdout"] += out
            result["stderr"] += err
            exit_code = stdout.channel.recv_exit_status()
            result["exit_code"] = exit_code
            
            if logger:
                logger.debug(f"Command {i} on {target_name} completed with exit_code={exit_code}")
            
            if exit_code != 0:
                result["success"] = False
                if logger:
                    logger.warning(f"Command {i} on {target_name} failed with exit_code={exit_code}")
                break
        
        if logger:
            logger.info(f"Command execution on {target_name} completed, success={result['success']}")
    except Exception as exc:  # pylint: disable=broad-except
        result["success"] = False
        error_msg = f"{type(exc).__name__}: {exc}"
        result["error"] = error_msg
        if logger:
            logger.error(f"Error executing commands on {target_name}: {error_msg}", exc_info=True)
    finally:
        if target_client:
            target_client.close()
        if bastion_client:
            bastion_client.close()
        if logger:
            logger.debug(f"Closed connections for {target_name}")

    return result


def worker(
    task_queue: "queue.Queue[Dict[str, Any]]",
    bastion: Dict[str, Any],
    commands: List[str],
    timeout: int,
    output: List[Dict[str, Any]],
):
    while True:
        try:
            target = task_queue.get_nowait()
        except queue.Empty:
            return
        result = run_commands(bastion, target, commands, timeout)
        output.append(result)
        task_queue.task_done()


def main() -> int:
    global logger
    
    parser = argparse.ArgumentParser(description="Execute commands via bastion using Paramiko.")
    parser.add_argument("--bastion", required=True, help="JSON payload for bastion connection info.")
    parser.add_argument("--targets", required=True, help="JSON array of target servers.")
    parser.add_argument("--commands", required=True, help="JSON array of commands to execute sequentially.")
    parser.add_argument("--concurrency", type=int, default=1, help="Max number of concurrent target connections.")
    parser.add_argument("--timeout", type=int, default=120, help="Timeout per SSH operation in seconds.")
    parser.add_argument("--log-level", default="INFO", choices=["DEBUG", "INFO", "WARNING", "ERROR"],
                        help="Log level (default: INFO)")
    parser.add_argument("--log-file", help="Log file path (optional)")
    args = parser.parse_args()

    # 初始化日志
    logger = setup_logging(args.log_level, args.log_file)

    bastion = json.loads(args.bastion)
    targets = json.loads(args.targets)
    commands = json.loads(args.commands)

    if not commands:
        raise ValueError("commands cannot be empty")
    if not targets:
        raise ValueError("targets cannot be empty")

    task_queue: "queue.Queue[Dict[str, Any]]" = queue.Queue()
    for target in targets:
        task_queue.put(target)

    results: List[Dict[str, Any]] = []
    threads: List[threading.Thread] = []
    worker_count = max(1, args.concurrency)

    for _ in range(worker_count):
        thread = threading.Thread(
            target=worker,
            args=(task_queue, bastion, commands, args.timeout, results),
            daemon=True,
        )
        thread.start()
        threads.append(thread)

    for thread in threads:
        thread.join()

    print(json.dumps(results, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())

