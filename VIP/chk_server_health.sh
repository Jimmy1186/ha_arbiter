#!/bin/bash

PORT="50000"

# 使用 curl 檢查
# -s: 靜音模式 (Silent)
# -f: 如果 HTTP 狀態碼 >= 400，curl 會回傳非 0 的結束碼 (Fail silently)
# -o /dev/null: 把輸出的內容丟掉，我們只需要知道「成功還是失敗」
curl -sf http://localhost:$PORT/health -o /dev/null

# $? 會抓取上一個指令 (curl) 的執行結果
# 0 代表成功 (HTTP 200)
# 非 0 代表失敗 (HTTP 4xx 或 5xx，或根本連不上)
if [ $? -eq 0 ]; then
    exit 0 # 告訴 Keepalived：我很健康
else
    exit 1 # 告訴 Keepalived：我掛了，快扣我分！
fi