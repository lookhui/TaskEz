# TaskEz

TaskEz v1.0 是一个基于 Go、Wails、React 构建的 Windows 应急巡检与响应工具。

它的目标是让使用者可以更快地查看主机当前状态，定位可疑进程、服务、启动项、外连和计划任务，并支持基础处置与离线分析。

## v1.0 功能

- 系统总览：CPU、内存、磁盘、网络、运行时间
- 进程树：父子进程关系、线程、模块
- 服务巡检：状态、启动类型、基础控制
- 驱动巡检：驱动状态、路径、类型
- 计划任务巡检：任务状态、作者、命令
- 网络巡检：连接、端口、进程映射
- 启动项巡检：Run、RunOnce、启动目录
- 基础响应：
  - 结束进程
  - 禁用启动项
  - 启动 / 停止 / 重启服务
  - 修改服务启动类型
- 离线分析包导出 / 导入
- 无 GUI 采集器，可在其他主机生成分析文件后导入本机查看

## 技术栈

- 后端：Go 1.24
- 桌面框架：Wails v2
- 前端：React + TypeScript + Vite

## 开发运行

```powershell
wails dev
```

## 构建桌面程序

```powershell
wails build
```

构建完成后主程序位于：

```text
build/bin/TaskEz.exe
```

## 构建无 GUI 采集器

```powershell
go build -tags collector -ldflags "-H=windowsgui" -o build/bin/TaskEzCollector.exe .
```

运行 `TaskEzCollector.exe` 后，会在当前目录生成一个 `.aldb` 分析包文件。

## 分析包说明

- GUI 中可通过设置窗口点击 `导出分析包`
- GUI 中可通过设置窗口点击 `导入分析包`
- 分析包扩展名：`.aldb`
- 分析包会经过压缩与应用内加密，主要用于 TaskEz 自身导入分析流程

## 说明

- 当前版本主要面向 Windows 主机
- 某些信息仍然可能受到本地权限影响
- v1.0 已具备基础巡检、基础响应与离线分析能力，但还不是所有底层取证工具的完全替代品
