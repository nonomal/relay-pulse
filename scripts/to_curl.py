from mitmproxy import http

def request(flow: http.HTTPFlow):
    # 获取请求对象
    req = flow.request

    # 1. 基础命令
    parts = [f"curl -X {req.method}"]

    # 2. 处理 Headers
    for key, value in req.headers.items():
        # 过滤掉一些 curl 会自动处理或不必要的 header，避免冗余
        if key.lower() in ["content-length", ":authority"]:
            continue
        parts.append(f"-H '{key}: {value}'")

    # 3. 处理 Body (如果有)
    if req.content:
        try:
            # 尝试读取文本内容
            body = req.get_text(strict=False)
            if body:
                # 简单转义单引号，防止 shell 报错
                body_escaped = body.replace("'", "'\\''")
                parts.append(f"-d '{body_escaped}'")
        except:
            # 如果是二进制文件，提示一下即可
            parts.append("# (Binary content ignored)")

    # 4. URL
    parts.append(f"'{req.url}'")

    # 5. 打印结果（绿色高亮方便查看）
    curl_cmd = " ".join(parts)
    print("\n" + "="*20 + " CURL COMMAND " + "="*20)
    print(f"\033[92m{curl_cmd}\033[0m")
    print("="*54 + "\n")


"""
mitmdump -p 8080 \
    --mode reverse:https://xxx.com \
    -s to_curl.py \
    --upstream http://192.168.1.31:8888
"""
