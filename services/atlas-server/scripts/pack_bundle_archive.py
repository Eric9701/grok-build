#!/usr/bin/env python3
"""把解压后的 bundle 目录打回 probe_bundle_archive。

线上格式：gzip，头里的原始文件名是 bundle.tar；解压 gzip 得到 tar，
tar 根上是 bundle.json / skills/ / subagents/ / workflows/。

默认读 services/atlas-server/probe_bundle_archive/（可带一层 bundle/ 包装），
写出 download/probe_bundle_archive。

    python scripts/pack_bundle_archive.py
    python scripts/pack_bundle_archive.py --src probe_bundle_archive --out download/probe_bundle_archive
"""

from __future__ import annotations

import argparse
import gzip
import io
import json
import sys
import tarfile
from pathlib import Path

# 与 crates/codegen/xai-grok-bundle extract_bundle_archive 上限对齐，超出 CLI 会拒收。
ARCHIVE_MAX_ENTRIES = 1000
ARCHIVE_MAX_ENTRY_SIZE = 1024 * 1024
ARCHIVE_MAX_DECOMPRESSED_SIZE = 50 * 1024 * 1024
SKIP_NAMES = {"manifest.json", ".ds_store", "thumbs.db"}
KEEP_TOP = ("bundle.json", "skills", "subagents", "workflows")


def atlas_server_root() -> Path:
    return Path(__file__).resolve().parent.parent


def payload_root(src: Path) -> Path:
    """允许 probe_bundle_archive/bundle/{bundle.json,skills,...} 或扁平布局。"""
    nested = src / "bundle" / "bundle.json"
    if nested.is_file():
        return src / "bundle"
    flat = src / "bundle.json"
    if flat.is_file():
        return src
    raise SystemExit(
        f"找不到 bundle.json：既不在 {src / 'bundle.json'} 也不在 {nested}"
    )


def should_keep(rel: str) -> bool:
    name = Path(rel).name.lower()
    if name in SKIP_NAMES or name.startswith("."):
        return False
    top = rel.split("/", 1)[0]
    return top in KEEP_TOP


def iter_files(root: Path) -> list[tuple[str, Path]]:
    out: list[tuple[str, Path]] = []
    for path in sorted(root.rglob("*")):
        if not path.is_file():
            continue
        rel = path.relative_to(root).as_posix()
        if should_keep(rel):
            out.append((rel, path))
    return out


def pack(src: Path, dest: Path) -> int:
    root = payload_root(src)
    files = iter_files(root)
    if not any(name == "bundle.json" for name, _ in files):
        raise SystemExit(f"{root} 缺少 bundle.json")

    version = "?"
    try:
        meta = json.loads((root / "bundle.json").read_text(encoding="utf-8"))
        version = str(meta.get("version") or "?")
    except (OSError, json.JSONDecodeError) as e:
        print(f"警告: 读 bundle.json 失败: {e}", file=sys.stderr)

    dest.parent.mkdir(parents=True, exist_ok=True)
    total = 0
    oversize: list[str] = []
    tar_buf = io.BytesIO()
    with tarfile.open(fileobj=tar_buf, mode="w") as tar:
        for rel, path in files:
            size = path.stat().st_size
            total += size
            if size > ARCHIVE_MAX_ENTRY_SIZE:
                oversize.append(f"{rel} ({size} bytes)")
            info = tar.gettarinfo(str(path), arcname=rel)
            info.uid = 0
            info.gid = 0
            info.uname = ""
            info.gname = ""
            with path.open("rb") as fh:
                tar.addfile(info, fh)
    tar_bytes = tar_buf.getvalue()

    dest_tmp = dest.with_name(dest.name + ".tmp")
    with dest_tmp.open("wb") as out:
        with gzip.GzipFile(
            filename="bundle.tar", mode="wb", fileobj=out, mtime=0
        ) as gz:
            gz.write(tar_bytes)
    dest_tmp.replace(dest)

    print(f"src        {root}")
    print(f"out        {dest}")
    print(f"version    {version}")
    print(f"files      {len(files)}")
    print(f"bundle.tar {len(tar_bytes)} bytes")
    print(f"payload    {total} bytes")
    print(f"gzip       {dest.stat().st_size} bytes (FNAME=bundle.tar)")
    prefixes: dict[str, int] = {}
    for rel, _ in files:
        prefixes[rel.split("/", 1)[0]] = prefixes.get(rel.split("/", 1)[0], 0) + 1
    print(f"layout     {prefixes}")

    failed = False
    if len(files) > ARCHIVE_MAX_ENTRIES:
        print(f"错误: 条目 {len(files)} > CLI 上限 {ARCHIVE_MAX_ENTRIES}", file=sys.stderr)
        failed = True
    if total > ARCHIVE_MAX_DECOMPRESSED_SIZE:
        print(
            f"错误: 解压大小 {total} > CLI 上限 {ARCHIVE_MAX_DECOMPRESSED_SIZE}",
            file=sys.stderr,
        )
        failed = True
    if oversize:
        print("错误: 下列文件超过 CLI 单文件 1MB 上限:", file=sys.stderr)
        for line in oversize:
            print(f"  {line}", file=sys.stderr)
        failed = True
    return 1 if failed else 0


def main(argv: list[str] | None = None) -> int:
    base = atlas_server_root()
    parser = argparse.ArgumentParser(
        description="把解压的 bundle 目录打成 gzip(bundle.tar) → download/probe_bundle_archive"
    )
    parser.add_argument(
        "--src",
        type=Path,
        default=base / "probe_bundle_archive",
        help="解压后的目录（默认同级 probe_bundle_archive/）",
    )
    parser.add_argument(
        "--out",
        type=Path,
        default=base / "download" / "probe_bundle_archive",
        help="输出的 gzip（默认 download/probe_bundle_archive）",
    )
    args = parser.parse_args(argv)
    src = args.src if args.src.is_absolute() else (Path.cwd() / args.src)
    out = args.out if args.out.is_absolute() else (Path.cwd() / args.out)
    if not src.is_dir():
        raise SystemExit(f"源目录不存在: {src}")
    return pack(src.resolve(), out.resolve())


if __name__ == "__main__":
    raise SystemExit(main())
