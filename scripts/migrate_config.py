#!/usr/bin/env python3
"""
config.yaml 通用迁移工具：老格式（anchor + url + !include）→ 新格式（template + base_url）

用法:
    pip install ruamel.yaml
    python3 scripts/migrate_config.py config.yaml --dry-run     # 分析+报告
    python3 scripts/migrate_config.py config.yaml --write       # 执行迁移
"""

import argparse
import json
import os
import re
import sys
from collections import defaultdict
from urllib.parse import urlsplit

try:
    from ruamel.yaml import YAML
    from ruamel.yaml.comments import CommentedMap, CommentedSeq
except ImportError:
    sys.stderr.write("未找到 ruamel.yaml，请先安装：pip install ruamel.yaml\n")
    sys.exit(1)

# --- 常量 ---

# !include 文件名 → template 名称（含历史模板的兼容映射）
INCLUDE_TO_TEMPLATE = {
    # 历史模板 → 最近似的 arith 模板
    "templates/cc-haiku-base.json": "cc-haiku-arith",
    "templates/cc-haiku-tiny.json": "cc-haiku-arith",
    "templates/cc-sonnet-tiny.json": "cc-sonnet-arith",
    "templates/cc-opus-tiny.json": "cc-opus-arith",
    "templates/cx-codex-base.json": "cx-codex-arith",
    "templates/cx-codexmini-base.json": "cx-codex-arith",
    "templates/cx-codexmax-base.json": "cx-codex-arith",
    "templates/cx-gpt52-base.json": "cx-codex-arith",
    "templates/gm-base.json": "gm-flash-arith",
    "templates/gm-thinking.json": "gm-flash-arith",
    "templates/gm-generate.json": "gm-flash-arith",
    # 当前模板
    "templates/cc-haiku-arith.json": "cc-haiku-arith",
    "templates/cc-sonnet-arith.json": "cc-sonnet-arith",
    "templates/cc-opus-arith.json": "cc-opus-arith",
    "templates/cx-gpt-arith.json": "cx-gpt-arith",
    "templates/cx-codex-arith.json": "cx-codex-arith",
    "templates/gm-flash-arith.json": "gm-flash-arith",
    "templates/gm-pro-arith.json": "gm-pro-arith",
    # 旧 arith 模板兼容
    "templates/cc-arith.json": "cc-haiku-arith",
    "templates/cx-arith.json": "cx-codex-arith",
    "templates/gm-arith.json": "gm-flash-arith",
}

# service 类型 → anchor 默认 template
SERVICE_DEFAULT_TEMPLATE = {
    "cc": "cc-haiku-arith",
    "cx": "cx-codex-arith",
    "gm": "gm-flash-arith",
}

# 旧 template 名称 → 新 arith 模板名称（用于 remap 已有 template: 字段的配置）
TEMPLATE_REMAP = {
    "cc-haiku-tiny": "cc-haiku-arith",
    "cc-sonnet-tiny": "cc-sonnet-arith",
    "cc-opus-tiny": "cc-opus-arith",
    "cc-arith": "cc-haiku-arith",
    "cx-codex-base": "cx-codex-arith",
    "cx-codexmax-base": "cx-codex-arith",
    "cx-codexmini-base": "cx-codex-arith",
    "cx-arith": "cx-codex-arith",
    "gm-base": "gm-flash-arith",
    "gm-thinking": "gm-flash-arith",
    "gm-generate": "gm-flash-arith",
    "gm-arith": "gm-flash-arith",
}

# 模板默认 slow_latency/timeout（与 JSON 模板中的值一致）
# 所有 arith 模板统一 5s/10s，无需特殊覆盖
TEMPLATE_DEFAULTS = {}
TEMPLATE_DEFAULT_SLOW = "5s"
TEMPLATE_DEFAULT_TIMEOUT = "10s"

# 顶层 anchor key
ANCHOR_KEYS = {"x-cc-template", "x-cx-template", "x-gm-template"}

# 正则
INCLUDE_RE = re.compile(r"^\s*!include\s+(templates/[A-Za-z0-9_./-]+\.json)\s*$")
GM_MODEL_RE = re.compile(r"/v1beta/models/([^/:?]+)")

# CC 标准路径后缀
CC_URL_SUFFIXES = ["/v1/messages", "/api/v1/messages"]
# CX 标准路径
CX_STANDARD_PATH = "/v1/responses"


# --- 工具函数 ---


def norm(value):
    """提取字符串值，strip 后返回；非字符串或空串返回 None。"""
    if not isinstance(value, str):
        return None
    s = value.strip()
    return s if s else None


def monitor_id(m):
    """构建 monitor 标识字符串，用于报告。"""
    provider = norm(m.get("provider")) or "(parent)"
    service = norm(m.get("service")) or "?"
    channel = norm(m.get("channel")) or ""
    model = norm(m.get("model")) or ""
    parent = norm(m.get("parent")) or ""
    parts = [provider, service]
    if channel:
        parts.append(channel)
    if model:
        parts.append(f"model={model}")
    if parent:
        parts.append(f"parent={parent}")
    return "/".join(parts)


# --- URL 解析 ---


def derive_base_url(url_value, service):
    """从完整 URL 中提取 base_url。"""
    parts = urlsplit(url_value)
    if not parts.scheme or not parts.netloc:
        return None, None
    base = f"{parts.scheme}://{parts.netloc}"
    path = parts.path or ""
    url_pattern = None

    if service == "cc":
        # 查找 /v1/messages 或 /api/v1/messages
        for suffix in CC_URL_SUFFIXES:
            idx = path.find(suffix)
            if idx != -1:
                prefix = path[:idx]
                # 有 query string → 需要 url_pattern
                if parts.query:
                    url_pattern = f"{{{{BASE_URL}}}}{path}"
                    if parts.query:
                        url_pattern += f"?{parts.query}"
                break
        else:
            # 未找到标准后缀，保留完整路径
            prefix = path
            url_pattern = url_value  # 非标准
        if prefix:
            base += prefix

    elif service == "cx":
        # 标准 /v1/responses 或非标准路径
        if path == CX_STANDARD_PATH or path == CX_STANDARD_PATH + "/":
            pass  # base 就是 scheme://netloc
        elif "/v1/responses" in path:
            prefix = path.split("/v1/responses", 1)[0]
            if prefix:
                base += prefix
        else:
            # 非标准路径（如 /responses, /codex/responses）
            url_pattern = f"{{{{BASE_URL}}}}{path}"
            if parts.query:
                url_pattern += f"?{parts.query}"

    elif service == "gm":
        # GM URL 含 /v1beta/models/xxx:streamGenerateContent
        marker = "/v1beta/models/"
        if marker in path:
            prefix = path.split(marker, 1)[0]
            if prefix:
                base += prefix
        else:
            url_pattern = url_value

    base = base.rstrip("/")
    return base, url_pattern


def extract_gm_model(url_value):
    """从 GM URL 中提取模型名。"""
    match = GM_MODEL_RE.search(url_value)
    if match:
        return match.group(1)
    return None


# --- 模板加载 ---


def load_template_json(data_dir, name, cache):
    """加载并缓存 JSON 模板文件。"""
    if name in cache:
        return cache[name]
    path = os.path.join(data_dir, f"{name}.json")
    try:
        with open(path, "r", encoding="utf-8") as f:
            tmpl = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        tmpl = None
    cache[name] = tmpl
    return tmpl


def template_slow_timeout(template_name, data_dir, template_cache):
    """获取模板的默认 slow_latency/timeout，优先从 JSON 文件读取。"""
    tmpl = load_template_json(data_dir, template_name, template_cache)
    if tmpl and isinstance(tmpl.get("response"), dict):
        resp = tmpl["response"]
        slow = norm(resp.get("slow_latency"))
        timeout = norm(resp.get("timeout"))
        if slow and timeout:
            return slow, timeout
    # 回退到硬编码默认值
    defaults = TEMPLATE_DEFAULTS.get(template_name, {})
    return (
        defaults.get("slow_latency", TEMPLATE_DEFAULT_SLOW),
        defaults.get("timeout", TEMPLATE_DEFAULT_TIMEOUT),
    )


# --- 单个 monitor 迁移 ---


def migrate_monitor(m, data_dir, template_cache, report):
    """迁移单个 monitor 条目，返回迁移类型字符串。"""
    if not isinstance(m, dict):
        return "skipped"

    has_parent = bool(norm(m.get("parent")))
    service = norm(m.get("service"))
    if not service and has_parent:
        # parent 子通道的 service 从 parent 路径中提取
        parent_val = norm(m.get("parent"))
        if parent_val and "/" in parent_val:
            parts = parent_val.split("/")
            if len(parts) >= 2:
                service = parts[1]
    service = (service or "").lower()

    # 如果已经有 template，先 remap 旧名称，再清理冗余字段
    existing_template = norm(m.get("template"))
    if existing_template:
        new_name = TEMPLATE_REMAP.get(existing_template)
        if new_name:
            m["template"] = new_name
            report["fields_removed"]["template_remapped"] += 1
            existing_template = new_name
        _cleanup_template_fields(m, existing_template, service, data_dir, template_cache, report)
        return "already_migrated"

    # --- Rule 3: body "!include" → template ---
    body_val = m.get("body")
    template_name = None

    if isinstance(body_val, str):
        inc_match = INCLUDE_RE.match(body_val)
        if inc_match:
            inc_path = inc_match.group(1)
            template_name = INCLUDE_TO_TEMPLATE.get(inc_path)
            if not template_name:
                # 尝试从文件名推导
                basename = os.path.splitext(os.path.basename(inc_path))[0]
                template_name = basename
                report["warnings"].append(
                    f"{monitor_id(m)}: 未知 !include '{inc_path}'，映射为 '{template_name}'"
                )

    # 无 !include 且有 anchor 展开的字段（method/headers 来自 <<: *alias）→ 用 service 默认模板
    # 判断依据：有 method+headers 但无 inline body（body 也是展开来的或不存在）
    has_anchor_fields = "method" in m and "headers" in m
    has_inline_body = isinstance(body_val, (dict, CommentedMap)) or (
        isinstance(body_val, str) and not INCLUDE_RE.match(body_val) and body_val.strip()
    )

    if not template_name and not has_parent:
        if has_anchor_fields and not has_inline_body:
            # anchor 展开，无自定义 body → 使用 service 默认模板
            template_name = SERVICE_DEFAULT_TEMPLATE.get(service)
        elif has_inline_body:
            # 有自定义 inline body（非 !include）→ 保留 method/headers/body，但仍做 url→base_url
            url_val = norm(m.get("url"))
            if url_val and not norm(m.get("base_url")):
                base, url_pattern = derive_base_url(url_val, service)
                if base:
                    m["base_url"] = base
                    if url_pattern:
                        m["url_pattern"] = url_pattern
                        report["url_patterns"].append(f"{monitor_id(m)}: {url_pattern}")
                    if "url" in m:
                        del m["url"]
                        report["fields_removed"]["url"] += 1
            report["warnings"].append(
                f"{monitor_id(m)}: 自定义 inline body，跳过模板化（需手动迁移）"
            )
            return "custom"

    # parent 子通道无 !include：不设 template（继承父通道的）
    if not template_name and has_parent:
        # 仍然清理展开的字段
        _remove_anchor_fields(m, report)
        return "parent_no_template"

    if not template_name:
        report["warnings"].append(f"{monitor_id(m)}: 无法确定 template")
        return "custom"

    m["template"] = template_name

    # --- Rule 2: url → base_url + model ---
    url_val = norm(m.get("url"))
    if url_val and not norm(m.get("base_url")):
        base, url_pattern = derive_base_url(url_val, service)
        if base:
            m["base_url"] = base
            if url_pattern:
                m["url_pattern"] = url_pattern
                report["url_patterns"].append(f"{monitor_id(m)}: {url_pattern}")
            if "url" in m:
                del m["url"]
                report["fields_removed"]["url"] += 1
        # GM 模型提取：仅对非独立通道提取（独立通道的 model 硬编码在 URL 中，由模板 {{MODEL}} 处理）
        # 注意：独立 GM 通道不设 model 字段，避免被误判为多模型父子结构

    # --- Rule 4: 删除模板已提供的字段 ---
    _remove_anchor_fields(m, report)

    # --- Rule 5: slow_latency/timeout 清理 ---
    tmpl_slow, tmpl_timeout = template_slow_timeout(template_name, data_dir, template_cache)
    if norm(m.get("slow_latency")) == tmpl_slow:
        del m["slow_latency"]
        report["fields_removed"]["slow_latency"] += 1
    if norm(m.get("timeout")) == tmpl_timeout:
        del m["timeout"]
        report["fields_removed"]["timeout"] += 1

    if has_parent:
        return "parent"
    return "migrated"


def _remove_anchor_fields(m, report):
    """删除从 anchor 展开来的字段（method/headers/body/success_contains）。"""
    for key in ("method", "headers", "body", "success_contains"):
        if key in m:
            del m[key]
            report["fields_removed"][key] += 1


def _cleanup_template_fields(m, template_name, service, data_dir, template_cache, report):
    """对已有 template 的 monitor 清理冗余字段，并处理 url→base_url。"""
    for key in ("method", "headers", "body", "success_contains"):
        if key in m:
            del m[key]
            report["fields_removed"][key] += 1

    # url → base_url 转换（已有 template 但仍保留 url 的情况）
    url_val = norm(m.get("url"))
    if url_val and not norm(m.get("base_url")) and service:
        base, url_pattern = derive_base_url(url_val, service)
        if base:
            m["base_url"] = base
            if url_pattern:
                m["url_pattern"] = url_pattern
                report["url_patterns"].append(f"{monitor_id(m)}: {url_pattern}")
            if "url" in m:
                del m["url"]
                report["fields_removed"]["url"] += 1

    # slow_latency/timeout 清理
    if template_name:
        tmpl_slow, tmpl_timeout = template_slow_timeout(template_name, data_dir, template_cache)
        if norm(m.get("slow_latency")) == tmpl_slow:
            del m["slow_latency"]
            report["fields_removed"]["slow_latency"] += 1
        if norm(m.get("timeout")) == tmpl_timeout:
            del m["timeout"]
            report["fields_removed"]["timeout"] += 1


def _has_merge(m):
    """检查 CommentedMap 是否有 merge key 残留。"""
    if not isinstance(m, CommentedMap):
        return False
    if "<<" in m:
        return True
    try:
        return hasattr(m, "_yaml_merge") and m._yaml_merge and len(m._yaml_merge) > 0
    except (AttributeError, TypeError):
        return False


def _rebuild_without_merge(m):
    """重建 CommentedMap，彻底去除 merge key 和内部 merge 状态。"""
    if not _has_merge(m):
        return m
    # 收集所有非 merge 的 key-value 对（m.keys() 已包含展开后的 key）
    items = [(k, m[k]) for k in m.keys()]
    new_m = CommentedMap(items)
    # 保留注释（如果有）
    if hasattr(m, "ca") and m.ca:
        try:
            new_m.ca = m.ca
        except (AttributeError, TypeError):
            pass
    return new_m


# --- 顶层迁移 ---


def migrate_config(doc, config_path):
    """执行完整迁移，返回 (doc, report)。"""
    report = {
        "total": 0,
        "migrated": 0,
        "already_migrated": 0,
        "parent": 0,
        "parent_no_template": 0,
        "custom": 0,
        "skipped": 0,
        "anchors_removed": [],
        "fields_removed": defaultdict(int),
        "url_patterns": [],
        "warnings": [],
    }

    if not isinstance(doc, dict):
        return doc, report

    # --- Rule 1: 删除顶层 anchor 定义 ---
    for key in list(ANCHOR_KEYS):
        if key in doc:
            del doc[key]
            report["anchors_removed"].append(key)

    monitors = doc.get("monitors")
    if not isinstance(monitors, (list, CommentedSeq)):
        return doc, report

    config_dir = os.path.dirname(os.path.abspath(config_path))
    data_dir = os.path.join(config_dir, "templates")
    template_cache = {}

    for i, m in enumerate(monitors):
        report["total"] += 1
        result = migrate_monitor(m, data_dir, template_cache, report)
        if result in report:
            report[result] += 1
        # 重建 CommentedMap 以彻底去除 merge key 残留
        rebuilt = _rebuild_without_merge(m)
        if rebuilt is not m:
            monitors[i] = rebuilt

    return doc, report


def print_report(report, file=sys.stderr):
    """输出迁移报告。"""
    print("\n=== Migration Report ===", file=file)
    print(f"Total monitors: {report['total']}", file=file)
    print(f"  Migrated: {report['migrated']} (anchor+url -> template+base_url)", file=file)
    print(f"  Already migrated: {report['already_migrated']} (had template, skipped)", file=file)
    print(f"  Parent channels: {report['parent']} (template from !include)", file=file)
    print(f"  Parent (no template): {report['parent_no_template']}", file=file)
    print(f"  Custom: {report['custom']}", file=file)
    print(f"  Skipped: {report['skipped']}", file=file)

    if report["anchors_removed"]:
        print(
            f"\nAnchor definitions removed: {len(report['anchors_removed'])} "
            f"({', '.join(report['anchors_removed'])})",
            file=file,
        )

    if report["fields_removed"]:
        print("\nFields removed:", file=file)
        for field, count in sorted(report["fields_removed"].items()):
            print(f"  {field}: {count}", file=file)

    if report["url_patterns"]:
        print(f"\nNon-standard URL patterns (url_pattern added): {len(report['url_patterns'])}", file=file)
        for p in report["url_patterns"]:
            print(f"  {p}", file=file)

    if report["warnings"]:
        print(f"\nWarnings ({len(report['warnings'])}):", file=file)
        for w in report["warnings"]:
            print(f"  {w}", file=file)

    print("", file=file)


# --- 入口 ---


def parse_args():
    parser = argparse.ArgumentParser(
        description="迁移 relay-pulse config.yaml 到模板化格式",
        epilog="示例: python3 scripts/migrate_config.py config.yaml --dry-run",
    )
    parser.add_argument("config", nargs="?", default="config.yaml", help="配置文件路径（默认 config.yaml）")
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--dry-run", action="store_true", help="输出迁移后的 YAML 到 stdout（默认模式）")
    mode.add_argument("--write", action="store_true", help="直接写回配置文件")
    return parser.parse_args()


def main():
    args = parse_args()
    dry_run = args.dry_run or not args.write

    yaml = YAML()
    yaml.preserve_quotes = True

    try:
        with open(args.config, "r", encoding="utf-8") as f:
            doc = yaml.load(f)
    except FileNotFoundError:
        sys.stderr.write(f"未找到配置文件: {args.config}\n")
        return 1

    doc, report = migrate_config(doc, args.config)
    print_report(report)

    if dry_run:
        yaml.dump(doc, sys.stdout)
    else:
        with open(args.config, "w", encoding="utf-8") as f:
            yaml.dump(doc, f)
        sys.stderr.write(f"已写入: {args.config}\n")

    return 0


if __name__ == "__main__":
    sys.exit(main())
