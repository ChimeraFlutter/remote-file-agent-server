#!/bin/bash

# 测试脚本 - 逐个测试每个端点，找出哪个卡住

SERVER="http://112.84.176.170:18120"
TIMEOUT=5

echo "========================================="
echo "开始测试各个端点"
echo "========================================="

# 测试1: Health检查
echo ""
echo "[1/5] 测试 /health 端点..."
START=$(date +%s%N)
RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
  --max-time $TIMEOUT \
  "$SERVER/health" 2>&1)
END=$(date +%s%N)
DURATION=$(echo "scale=3; ($END - $START) / 1000000000" | bc)
echo "响应: $RESPONSE"
echo "耗时: ${DURATION}秒"

# 测试2: 登录端点
echo ""
echo "[2/5] 测试 /api/admin/login 端点..."
START=$(date +%s%N)
RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
  --max-time $TIMEOUT \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' \
  "$SERVER/api/admin/login" 2>&1)
END=$(date +%s%N)
DURATION=$(echo "scale=3; ($END - $START) / 1000000000" | bc)
echo "响应: $RESPONSE"
echo "耗时: ${DURATION}秒"

# 提取token（如果登录成功）
TOKEN=$(echo "$RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if [ -n "$TOKEN" ]; then
  echo "登录成功，Token: $TOKEN"

  # 测试3: 获取设备列表
  echo ""
  echo "[3/5] 测试 /api/admin/devices 端点..."
  START=$(date +%s%N)
  RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
    --max-time $TIMEOUT \
    -H "Authorization: Bearer $TOKEN" \
    "$SERVER/api/admin/devices" 2>&1)
  END=$(date +%s%N)
  DURATION=$(echo "scale=3; ($END - $START) / 1000000000" | bc)
  echo "响应: $RESPONSE"
  echo "耗时: ${DURATION}秒"

  # 测试4: 获取审计日志
  echo ""
  echo "[4/5] 测试 /api/admin/audit-logs 端点..."
  START=$(date +%s%N)
  RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}\nTIME:%{time_total}" \
    --max-time $TIMEOUT \
    -H "Authorization: Bearer $TOKEN" \
    "$SERVER/api/admin/audit-logs?limit=10" 2>&1)
  END=$(date +%s%N)
  DURATION=$(echo "scale=3; ($END - $START) / 1000000000" | bc)
  echo "响应: $RESPONSE"
  echo "耗时: ${DURATION}秒"
else
  echo "登录失败，跳过后续测试"
fi

# 测试5: 并发测试 - 同时发送多个请求
echo ""
echo "[5/5] 并发测试 - 同时发送5个登录请求..."
for i in {1..5}; do
  (
    START=$(date +%s%N)
    RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
      --max-time $TIMEOUT \
      -X POST \
      -H "Content-Type: application/json" \
      -d '{"username":"admin","password":"admin123"}' \
      "$SERVER/api/admin/login" 2>&1)
    END=$(date +%s%N)
    DURATION=$(echo "scale=3; ($END - $START) / 1000000000" | bc)
    echo "请求 $i 完成，耗时: ${DURATION}秒"
  ) &
done

wait
echo ""
echo "========================================="
echo "测试完成"
echo "========================================="
