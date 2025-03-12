#!/bin/bash

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Caddy-Gen 功能测试${NC}"
echo "=============================="

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
  echo -e "${RED}错误: Docker 未运行。请先启动 Docker。${NC}"
  exit 1
fi

# 创建测试目录
mkdir -p sites

# 启动测试环境
echo -e "\n${YELLOW}1. 启动测试环境${NC}"
docker-compose down -v > /dev/null 2>&1
docker-compose up -d
if [ $? -ne 0 ]; then
  echo -e "${RED}错误: 无法启动测试环境。${NC}"
  exit 1
fi
echo -e "${GREEN}测试环境已启动。${NC}"

# 等待服务启动
echo -e "\n${YELLOW}2. 等待服务启动${NC}"
echo "等待 10 秒..."
sleep 10

# 检查 caddy-gen 是否生成了配置文件
echo -e "\n${YELLOW}3. 检查配置文件${NC}"
if [ -f "sites/docker-sites.caddy" ]; then
  echo -e "${GREEN}配置文件已生成: sites/docker-sites.caddy${NC}"
  echo "配置文件内容:"
  echo "-----------------------------"
  cat sites/docker-sites.caddy
  echo "-----------------------------"
else
  echo -e "${RED}错误: 配置文件未生成。${NC}"
  docker-compose logs caddy-gen
  exit 1
fi

# 测试动态更新
echo -e "\n${YELLOW}4. 测试动态更新${NC}"
echo "停止 web1 容器..."
docker-compose stop web1
echo "等待 5 秒..."
sleep 5

echo "检查配置文件是否更新..."
grep -q "web1.local" sites/docker-sites.caddy
if [ $? -eq 0 ]; then
  echo -e "${RED}错误: web1.local 仍然存在于配置文件中。${NC}"
else
  echo -e "${GREEN}配置文件已正确更新，web1.local 已移除。${NC}"
fi

echo "重新启动 web1 容器..."
docker-compose start web1
echo "等待 5 秒..."
sleep 5

echo "检查配置文件是否更新..."
grep -q "web1.local" sites/docker-sites.caddy
if [ $? -eq 0 ]; then
  echo -e "${GREEN}配置文件已正确更新，web1.local 已添加回来。${NC}"
else
  echo -e "${RED}错误: web1.local 未添加回配置文件。${NC}"
fi

# 测试完成
echo -e "\n${YELLOW}5. 测试完成${NC}"
echo -e "${GREEN}所有测试已完成。${NC}"
echo -e "你可以通过以下方式访问测试网站（需要先修改 hosts 文件）:"
echo "- http://web1.local"
echo "- http://web2.local/api"
echo "- http://web3.local"
echo "- http://www.web3.local"

echo -e "\n${YELLOW}提示:${NC} 使用 'docker-compose down -v' 清理测试环境。" 