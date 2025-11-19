#!/usr/bin/env python3
"""
Executor script that connects to target servers via a bastion host using Paramiko.
The script expects JSON payloads passed through CLI arguments so that the Go
service can invoke it securely.
"""

import argparse
import json
import queue
import sys
import threading
from typing import Any, Dict, List

import paramiko


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
        raise RuntimeError("unable to obtain bastion transport")

    dest_addr = (target["host"], target.get("port", 22))
    local_addr = ("127.0.0.1", 0)
    channel = transport.open_channel("direct-tcpip", dest_addr, local_addr)

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

    try:
        bastion_client, target_client = connect_via_bastion(bastion, target, timeout)
        for command in commands:
            stdin, stdout, stderr = target_client.exec_command(command, timeout=timeout)
            out = stdout.read().decode(errors="ignore")
            err = stderr.read().decode(errors="ignore")
            result["stdout"] += out
            result["stderr"] += err
            exit_code = stdout.channel.recv_exit_status()
            result["exit_code"] = exit_code
            if exit_code != 0:
                result["success"] = False
                break
    except Exception as exc:  # pylint: disable=broad-except
        result["success"] = False
        result["error"] = f"{type(exc).__name__}: {exc}"
    finally:
        if target_client:
            target_client.close()
        if bastion_client:
            bastion_client.close()

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
    parser = argparse.ArgumentParser(description="Execute commands via bastion using Paramiko.")
    parser.add_argument("--bastion", required=True, help="JSON payload for bastion connection info.")
    parser.add_argument("--targets", required=True, help="JSON array of target servers.")
    parser.add_argument("--commands", required=True, help="JSON array of commands to execute sequentially.")
    parser.add_argument("--concurrency", type=int, default=1, help="Max number of concurrent target connections.")
    parser.add_argument("--timeout", type=int, default=120, help="Timeout per SSH operation in seconds.")
    args = parser.parse_args()

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

