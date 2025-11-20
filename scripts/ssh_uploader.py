#!/usr/bin/env python3
"""
File upload script that connects to target servers via a bastion host using Paramiko.
The script uploads files from local directory to remote servers.
"""

import argparse
import json
import os
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
        "uploaded_files": [],
        "failed_files": [],
        "error": "",
    }


def connect_via_bastion(bastion: Dict[str, Any], target: Dict[str, Any], timeout: int):
    """Connect to target server via bastion host."""
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


def upload_files(
    bastion: Dict[str, Any],
    target: Dict[str, Any],
    local_path: str,
    remote_path: str,
    timeout: int,
) -> Dict[str, Any]:
    """Upload files from local directory to remote server."""
    result = build_result(target)
    bastion_client = None
    target_client = None
    sftp = None

    try:
        bastion_client, target_client = connect_via_bastion(bastion, target, timeout)
        sftp = target_client.open_sftp()

        # 确保本地路径存在
        if not os.path.exists(local_path):
            result["success"] = False
            result["error"] = f"local path does not exist: {local_path}"
            return result

        # 确保远程目录存在
        try:
            sftp.stat(remote_path)
        except FileNotFoundError:
            # 远程目录不存在，尝试创建
            try:
                sftp.mkdir(remote_path)
            except Exception as e:
                result["success"] = False
                result["error"] = f"failed to create remote directory: {e}"
                return result

        uploaded = []
        failed = []

        # 如果是文件，直接上传
        if os.path.isfile(local_path):
            remote_file_path = os.path.join(remote_path, os.path.basename(local_path)).replace("\\", "/")
            try:
                sftp.put(local_path, remote_file_path)
                uploaded.append({"local": local_path, "remote": remote_file_path})
            except Exception as e:
                failed.append({"local": local_path, "remote": remote_file_path, "error": str(e)})
                result["success"] = False
        # 如果是目录，递归上传
        elif os.path.isdir(local_path):
            for root, dirs, files in os.walk(local_path):
                # 计算相对路径
                rel_path = os.path.relpath(root, local_path)
                if rel_path == ".":
                    remote_dir = remote_path
                else:
                    remote_dir = os.path.join(remote_path, rel_path).replace("\\", "/")

                # 确保远程目录存在
                try:
                    sftp.stat(remote_dir)
                except FileNotFoundError:
                    try:
                        sftp.mkdir(remote_dir)
                    except Exception as e:
                        failed.append({"local": root, "remote": remote_dir, "error": f"failed to create directory: {e}"})
                        result["success"] = False
                        continue

                # 上传文件
                for file in files:
                    local_file = os.path.join(root, file)
                    remote_file = os.path.join(remote_dir, file).replace("\\", "/")
                    try:
                        sftp.put(local_file, remote_file)
                        uploaded.append({"local": local_file, "remote": remote_file})
                    except Exception as e:
                        failed.append({"local": local_file, "remote": remote_file, "error": str(e)})
                        result["success"] = False

        result["uploaded_files"] = uploaded
        result["failed_files"] = failed

        if len(failed) > 0:
            result["success"] = False
            if not result["error"]:
                result["error"] = f"{len(failed)} file(s) failed to upload"

    except Exception as exc:  # pylint: disable=broad-except
        result["success"] = False
        result["error"] = f"{type(exc).__name__}: {exc}"
    finally:
        if sftp:
            sftp.close()
        if target_client:
            target_client.close()
        if bastion_client:
            bastion_client.close()

    return result


def worker(
    task_queue: "queue.Queue[Dict[str, Any]]",
    bastion: Dict[str, Any],
    local_path: str,
    remote_path: str,
    timeout: int,
    output: List[Dict[str, Any]],
):
    """Worker thread for concurrent file uploads."""
    while True:
        try:
            target = task_queue.get_nowait()
        except queue.Empty:
            return
        result = upload_files(bastion, target, local_path, remote_path, timeout)
        output.append(result)
        task_queue.task_done()


def main() -> int:
    parser = argparse.ArgumentParser(description="Upload files via bastion using Paramiko.")
    parser.add_argument("--bastion", required=True, help="JSON payload for bastion connection info.")
    parser.add_argument("--targets", required=True, help="JSON array of target servers.")
    parser.add_argument("--local-path", required=True, help="Local file or directory path to upload.")
    parser.add_argument("--remote-path", required=True, help="Remote directory path on target servers.")
    parser.add_argument("--concurrency", type=int, default=1, help="Max number of concurrent target connections.")
    parser.add_argument("--timeout", type=int, default=120, help="Timeout per SSH operation in seconds.")
    args = parser.parse_args()

    bastion = json.loads(args.bastion)
    targets = json.loads(args.targets)

    if not targets:
        raise ValueError("targets cannot be empty")

    if not os.path.exists(args.local_path):
        raise ValueError(f"local path does not exist: {args.local_path}")

    task_queue: "queue.Queue[Dict[str, Any]]" = queue.Queue()
    for target in targets:
        task_queue.put(target)

    results: List[Dict[str, Any]] = []
    threads: List[threading.Thread] = []
    worker_count = max(1, args.concurrency)

    for _ in range(worker_count):
        thread = threading.Thread(
            target=worker,
            args=(task_queue, bastion, args.local_path, args.remote_path, args.timeout, results),
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

