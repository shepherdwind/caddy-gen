# Caddy-Gen 测试示例

这个目录包含一个完整的测试环境，用于验证 caddy-gen 的功能。

## 测试环境

测试环境包含以下组件：

1. **Caddy 服务器** - 用于提供 HTTP/HTTPS 服务
2. **caddy-gen** - 监控 Docker 容器并生成 Caddy 配置
3. **测试网站** - 三个简单的 Nginx 网站，用于测试不同的配置场景：
   - **web1** - 简单的主机名配置
   - **web2** - 带路径和自定义头的配置
   - **web3** - 带多个主机名和多种指令的配置

## 使用方法

### 1. 启动测试环境

```bash
cd examples
docker-compose up -d
```

### 2. 修改本地 hosts 文件

将以下内容添加到你的 `/etc/hosts` 文件中：

```
127.0.0.1 web1.local web2.local web3.local www.web3.local
```

### 3. 访问测试网站

- http://web1.local - 简单的网站
- http://web2.local/api - 带路径的网站
- http://web3.local - 带多个主机名和指令的网站
- http://www.web3.local - web3 的别名

### 4. 查看生成的配置

```bash
cat sites/docker-sites.caddy
```

### 5. 测试动态更新

尝试添加、修改或删除容器，然后观察 caddy-gen 如何自动更新配置：

```bash
# 停止一个网站
docker-compose stop web1

# 启动一个网站
docker-compose start web1

# 修改标签并重新创建
docker-compose up -d --force-recreate web2
```

## 清理

```bash
docker-compose down -v
```
