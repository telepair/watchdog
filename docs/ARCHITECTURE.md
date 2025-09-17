# Watchdog 监控平台架构设计文档

## 目录

- [1. 系统概述](#1-系统概述)
- [2. 架构设计](#2-架构设计)
- [3. 技术选型](#3-技术选型)
- [4. 核心模块](#4-核心模块)
- [5. 数据模型](#5-数据模型)
- [6. 安全架构](#6-安全架构)
- [8. 安全架构](#8-安全架构)
- [9. 性能设计](#9-性能设计)
- [10. 可观测性设计](#10-可观测性设计)
- [11. 扩展性设计](#11-扩展性设计)
- [12. 容灾与高可用设计](#12-容灾与高可用设计)

## 1. 系统概述

### 1.1 项目愿景

**Watchdog** 是一个面向小团队和个人开发者的轻量级监控平台，致力于提供**开箱即用、零依赖部署**的监控解决方案。通过模板驱动的配置和垂直场景的深度集成，让监控变得简单而强大。

### 1.2 核心特性

- **🚀 零配置启动**: 单二进制部署，内嵌所有依赖
- **📊 全栈监控**: HTTP/TCP/系统资源/应用指标全覆盖
- **🔔 智能告警**: 多维度告警策略与生命周期管理
- **🎯 模板驱动**: 参数化监控模板，垂直场景深度集成
- **🛡️ 企业级安全**: RBAC 权限控制，数据加密存储
- **📈 高性能**: 单机支持 1000+监控目标，毫秒级响应
- **🤖 自动化运维**: 智能指令执行、任务调度、协作终端
- **🌐 Agent 架构**: 轻量级 Agent，跨平台支持，安全通信
- **🔗 开放集成**: Webhook 推送、Prometheus 兼容、K8s 深度集成

### 1.3 架构原则

- **模块化设计**: 低耦合、高内聚的组件架构
- **分层架构**: 清晰的职责分离与依赖关系
- **插件化扩展**: 开放的扩展点与插件生态
- **云原生友好**: 容器化、可观测、易扩展
- **数据驱动**: 基于指标的决策与自动化

## 2. 架构设计

### 2.1 整体架构

#### 系统总体架构图

```text
┌─────────────────────────────────────────────────────────────────────┐
│                           Watchdog Single Node                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    接入层 (Access Layer)                    │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐       │   │
│  │  │   Web UI     │ │  REST API    │ │  WebSocket   │       │   │
│  │  │   (HTMX)     │ │   (JSON)     │ │ (Real-time)  │       │   │
│  │  └──────────────┘ └──────────────┘ └──────────────┘       │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                ↓                                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                   应用服务层 (Service Layer)                │   │
│  │ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐ │   │
│  │ │Auth Service│ │Rule Engine │ │Alert Mgr   │ │Config Mgr │ │   │
│  │ └────────────┘ └────────────┘ └────────────┘ └───────────┘ │   │
│  │ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌───────────┐ │   │
│  │ │Query Svc   │ │Notify Svc  │ │Template Mgr│ │Dashboard  │ │   │
│  │ └────────────┘ └────────────┘ └────────────┘ └───────────┘ │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                ↓                                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                   核心引擎层 (Core Engine)                   │   │
│  │ ┌─────────────────────────────────────────────────────────┐ │   │
│  │ │          Collection Framework (采集框架)               │ │   │
│  │ │ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────┐ │ │   │
│  │ │ │HTTP/API │ │Ping/TCP │ │Scripts  │ │Prom     │ │K8s   │ │ │   │
│  │ │ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └──────┘ │ │   │
│  │ │ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────┐ │ │   │
│  │ │ │Agent    │ │Push/WH  │ │Template │ │Command  │ │File  │ │ │   │
│  │ │ │Manager  │ │Receiver │ │Engine   │ │Executor │ │Watcher│ │ │   │
│  │ │ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └──────┘ │ │   │
│  │ └─────────────────────────────────────────────────────────┘ │   │
│  │ ┌─────────────────────────────────────────────────────────┐ │   │
│  │ │        Notification Framework (通知框架)              │ │   │
│  │ │ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────┐ │ │   │
│  │ │ │Telegram │ │Email    │ │Webhook  │ │Slack    │ │WeChat│ │ │   │
│  │ │ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └──────┘ │ │   │
│  │ └─────────────────────────────────────────────────────────┘ │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                ↓                                    │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                   存储层 (Storage Layer)                    │   │
│  │ ┌──────────────────┐ ┌──────────────────┐ ┌───────────────┐ │   │
│  │ │VictoriaMetrics   │ │    SQLite        │ │ Memory Cache  │ │   │
│  │ │ (Time Series)    │ │ (Config & Meta)  │ │ (Ristretto)   │ │   │
│  │ └──────────────────┘ └──────────────────┘ └───────────────┘ │   │
│  │ ┌──────────────────┐ ┌──────────────────┐ ┌───────────────┐ │   │
│  │ │File System       │ │NATS Embedded     │ │Structured Logs│ │   │
│  │ │(Assets & Config) │ │(Message Bus)     │ │(Audit Trail)  │ │   │
│  │ └──────────────────┘ └──────────────────┘ └───────────────┘ │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 分层架构详述

#### 接入层 (Access Layer)

负责外部请求接入和协议转换：

- **Web UI**: 基于 HTMX + TailwindCSS 的现代化管理界面
- **REST API**: 标准 RESTful API，支持 JSON/YAML 格式
- **WebSocket**: 实时数据推送和协作终端
- **Webhook Receiver**: 外部系统集成和事件接收
- **Push Gateway**: 数据推送接口，支持安全认证

#### 应用服务层 (Service Layer)

业务逻辑和服务编排：

- **Auth Service**: 统一认证和授权管理，支持 RBAC
- **Rule Engine**: 告警规则评估和状态管理
- **Alert Manager**: 告警生命周期管理和智能降噪
- **Config Manager**: 配置版本控制和热重载
- **Query Service**: 统一查询接口和缓存
- **Notification Service**: 多渠道通知路由和发送
- **Template Manager**: 模板管理和实例化，支持垂直场景
- **Dashboard Service**: 仪表盘和可视化引擎
- **Agent Manager**: Agent 生命周期管理和通信
- **Command Executor**: 远程指令执行和任务调度
- **Terminal Service**: 协作终端和会话管理

#### 核心引擎层 (Core Engine)

核心功能实现：

- **Collection Framework**: 可扩展的采集框架，支持多协议
- **Notification Framework**: 可扩展的通知框架，支持多渠道
- **Template Engine**: 模板引擎，支持参数化和实例化
- **Command Engine**: 指令执行引擎，支持安全约束
- **Plugin System**: 插件加载和生命周期管理
- **Scheduler**: 任务调度和资源管理，支持 Cron 表达式
- **Security Engine**: 安全引擎，负责数据加密和访问控制

#### 存储层 (Storage Layer)

数据持久化和缓存：

- **VictoriaMetrics**: 高性能时序数据库，支持 Prometheus 兼容
- **SQLite**: 轻量级关系数据库（配置、用户、规则）
- **Memory Cache**: 高速内存缓存（Ristretto）
- **File System**: 配置文件、模板和静态资源
- **NATS Embedded**: 内嵌消息总线，用于 Agent 通信
- **Structured Logs**: 结构化日志存储（审计、操作）
- **Backup Storage**: 备份存储，支持本地和云存储

### 2.3 Agent 架构设计

#### Agent 系统架构

```text
┌──────────────────────────────────────────────────────┐
│                    Watchdog Server                     │
│ ┌─────────────────────────────────────────────────┐ │
│ │                Agent Manager                │ │
│ │  ● Agent Registry      ● Command Queue     │ │
│ │  ● Health Monitor      ● Task Scheduler    │ │
│ │  ● Permission Control ● Result Collector  │ │
│ └─────────────────────────────────────────────────┘ │
└───────────────────────┬──────────────────────────────┘
                       │ NATS/gRPC (TLS)
┌───────────────────────┼──────────────────────────────┐
│              Watchdog Agent               │
│ ┌───────────────────────────────────────────┐ │
│ │            Core Services             │ │
│ │ ┌─────────────┐ ┌──────────────────┐ │ │
│ │ │System Monitor │ │ Command Executor │ │ │
│ │ │• CPU/Memory   │ │ • Script Engine  │ │ │
│ │ │• Disk/Network │ │ • Security Check │ │ │
│ │ │• Process List │ │ • Resource Limit│ │ │
│ │ └─────────────┘ └──────────────────┘ │ │
│ │ ┌─────────────┐ ┌──────────────────┐ │ │
│ │ │File Manager   │ │ Terminal Proxy   │ │ │
│ │ │• Upload/Down   │ │ • Shell Access    │ │ │
│ │ │• Permission    │ │ • Session Record  │ │ │
│ │ │• Path Control  │ │ • Multi-user      │ │ │
│ │ └─────────────┘ └──────────────────┘ │ │
│ └───────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

#### Agent 核心特性

- **轻量级设计**: <50MB 内存，<1% CPU，单二进制郦署
- **跨平台支持**: Linux/Windows/macOS/ARM 全支持
- **安全通信**: TLS 1.3 + Token 认证，端到端加密
- **智能发现**: 自动发现系统服务和资源
- **权限分级**: 监控、运维、管理三级权限体系

### 2.4 模板架构设计

#### 模板引擎架构

```text
┌───────────────────────────────────────────────────────────┐
│                    Template Management System                     │
│ ┌────────────────────────────┐ ┌────────────────────────────┐ │
│ │       Template Store        │ │     Parameter Engine      │ │
│ │  ● Official Templates     │ │  ● Variable Replacement   │ │
│ │  ● Community Templates   │ │  ● Condition Logic       │ │
│ │  ● Custom Templates      │ │  ● Form Generation       │ │
│ │  ● Version Control       │ │  ● Validation            │ │
│ └────────────────────────────┘ └────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │                 Vertical Scene Templates                   │ │
│ │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ │ │
│ │ │Cryptocurrency│ │ Stock Market  │ │ Weather/IoT   │ │ │
│ │ │• Price Alert  │ │ • Technical   │ │ • Temperature │ │ │
│ │ │• Volume Track │ │ • PE Ratio     │ │ • Air Quality │ │ │
│ │ │• Market Cap   │ │ • Earnings    │ │ • Humidity    │ │ │
│ │ └─────────────┘ └─────────────┘ └─────────────┘ │ │
│ │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ │ │
│ │ │System Monitor│ │ Web Service   │ │ Database      │ │ │
│ │ │• CPU/Memory  │ │ • HTTP Check  │ │ • Connection  │ │ │
│ │ │• Disk Space  │ │ • SSL Cert    │ │ • Performance │ │ │
│ │ │• Network     │ │ • Load Time   │ │ • Backup      │ │ │
│ │ └─────────────┘ └─────────────┘ └─────────────┘ │ │
│ └─────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────┘
```

#### 垂直场景模板库

**加密货币监控模板**:

- 数据源: CoinGecko、Binance、CoinMarketCap
- 监控指标: 价格、成交量、市值排名、波动率
- 告警策略: 价格阈值、波动异常、恢慌贪婪指数
- 配置参数: 币种选择、阈值设置、通知方式

**股票投资监控模板**:

- 数据源: Yahoo Finance、Alpha Vantage、东方财富
- 监控指标: 价格、RSI、MACD、PE 比率、成交量
- 告警策略: 技术指标、财报日期、业绩预警
- 配置参数: 股票代码、指标参数、阅值范围

**气象环境监控模板**:

- 数据源: OpenWeatherMap、AccuWeather、中国气象局
- 监控指标: 温度、湿度、空气质量、极端天气
- 告警策略: 极端天气预警、空气质量超标、温度异常
- 配置参数: 地理位置、关注指标、阔值设定

## 3. 技术选型

### 3.1 技术栈概览

| 技术领域       | 选择方案        | 替代方案       | 选择理由 |
| -------------- | --------------- | -------------- | -------- |
| **编程语言**   | Go 1.25+        | Rust, Java     | 并发性能 |
| **Web 框架**   | Gin             | Echo, Fiber    | 成熟生态 |
| **ORM**        | Ent             | GORM           | 类型安全 |
| **模板引擎**   | Templ           | html/template  | 类型安全 |
| **前端技术**   | HTMX+Tailwind   | React/Vue      | SSR      |
| **关系数据库** | SQLite          | PostgreSQL     | 零配置   |
| **时序数据库** | VictoriaMetrics | InfluxDB       | 高性能   |
| **消息队列**   | NATS Embedded   | Redis,RabbitMQ | 轻量级   |
| **缓存**       | Ristretto       | BigCache       | 高性能   |

### 2.5 自动化运维架构

#### 指令执行引擎

```text
┌───────────────────────────────────────────────────────────┐
│                    Command Execution Engine                     │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │                    Security Layer                     │ │
│ │ ┌─────────────────────────────────────────────────┐ │ │
│ │ │ ● Permission Control  ● Resource Limits      │ │ │
│ │ │ ● Command Whitelist   ● Execution Timeout    │ │ │
│ │ │ ● User Authentication ● Audit Logging       │ │ │
│ │ └─────────────────────────────────────────────────┘ │ │
│ │ ┌─────────────────────────────────────────────────┐ │ │
│ │ │                Execution Modes                │ │ │
│ │ │ ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │ │ │
│ │ │ │ Immediate   │ │ Scheduled   │ │ Triggered  │ │ │ │
│ │ │ │ Execution   │ │ Tasks       │ │ Response   │ │ │ │
│ │ │ └─────────────┘ └─────────────┘ └────────────┘ │ │ │
│ │ └─────────────────────────────────────────────────┘ │ │
│ └─────────────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │                   Task Scheduler                     │ │
│ │ ● Cron Expression Support  ● Dependency Management      │ │
│ │ ● Fixed Interval Tasks     ● Retry Logic              │ │
│ │ ● One-time Tasks          ● Performance Analytics   │ │
│ └─────────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────┘
```

#### 协作终端架构

```text
┌───────────────────────────────────────────────────────────┐
│                 Collaborative Terminal System                 │
┌──────────────┼────────────────────────────────────────┼──────────────┐
│   Web Client   │              Server Core              │   Agent Side   │
│               │                                      │               │
│● Browser      │ ┌──────────────────────────────────────┐ │● PTY Wrapper   │
│● Terminal UI  │ │          Session Manager          │ │● Shell Process │
│● File Upload  │ │ ● Multi-user Sessions          │ │● File Transfer│
│● Real-time    │ │ ● Permission Control           │ │● Command Log  │
│  Sync         │ │ ● Session Recording            │ │● Resource     │
│               │ │ ● Resource Monitoring          │ │  Monitoring   │
│               │ └──────────────────────────────────────┘ │               │
└───────────────┼────────────────────────────────────────┼──────────────┘
               │        WebSocket (WSS/TLS)          │
               └──────────────────────────────────────┘
```

### 2.6 Push 数据与 Webhook 架构

#### 数据推送架构

```text
┌───────────────────────────────────────────────────────────┐
│              External Data Sources & Services              │
│ ┌───────────────────────────────────────────────────────┐ │
│ │   GitHub   │  GitLab   │ Jenkins  │ Custom App │ │
│ │ Webhooks  │ Webhooks  │ Pipeline │  Events    │ │
│ └───────────────────────────────────────────────────────┘ │
└────────────────────────────┬──────────────────────────────┘
                           │ HTTPS/TLS
┌────────────────────────────┼──────────────────────────────┐
│         Watchdog Push Gateway         │
│ ┌───────────────────────────────────────────────────────┐ │
│ │                Ingress Layer                │ │
│ │ ● HMAC-SHA256 Signature Verification       │ │
│ │ ● Token-based Authentication              │ │
│ │ ● Rate Limiting & DDoS Protection         │ │
│ │ ● Content Type Detection                 │ │
│ └───────────────────────────────────────────────────────┘ │
│ ┌───────────────────────────────────────────────────────┐ │
│ │              Processing Layer               │ │
│ │ ● Smart Data Routing                     │ │
│ │ ● Format Auto-detection (JSON/XML/Form)  │ │
│ │ ● Deduplication & Buffering             │ │
│ │ ● Reliable Retry Mechanism              │ │
│ └───────────────────────────────────────────────────────┘ │
└───────────────────────────────────────────────────────────┘
                           │
                           ▼
┌───────────────────────────────────────────────────────────┐
│              Core Monitoring System              │
│        Alert Engine │ Storage │ Dashboard        │
└───────────────────────────────────────────────────────────┘
```

#### 支持的 Webhook 接口

**通用 Push 接口**:

```text
POST https://watchdog.example.com/api/v1/push/{source_id}/{token}
```

**主流平台支持**:

- **GitHub**: Issues, Pull Requests, Push Events, Workflow Status
- **GitLab**: Pipeline Status, Merge Requests, Issue Updates
- **Jenkins**: Build Status, Test Results, Deployment Events
- **Prometheus**: Alert Manager Integration
- **Grafana**: Annotation Events, Alert Status

### 3.2 架构决策记录 (ADR)

### 3.3 版本规划与技术差异

#### 三版本技术对比

| 功能领域     | 开源版 (Community)       | 企业版 (Enterprise)                  | 云服务 (SaaS)       |
| ------------ | ------------------------ | ------------------------------------ | ------------------- |
| **部署架构** | 单机部署，内嵌存储       | 高可用集群，分布式存储               | 多租户 SaaS 架构    |
| **数据存储** | SQLite + VictoriaMetrics | PostgreSQL + VictoriaMetrics Cluster | 云原生数据库        |
| **消息队列** | NATS Embedded            | NATS Cluster                         | 云消息服务          |
| **用户管理** | 单用户模式               | 多租户 + RBAC + SSO                  | 团队协作 + 企业集成 |
| **数据保留** | 自定义配置               | 无限制自定义                         | 1 年+ 自动管理      |
| **高级特性** | 基础 AI 特性             | 高级 AI + 机器学习                   | 云原生 AI 服务      |
| **性能扩展** | 垂直扩展                 | 水平 + 垂直扩展                      | 弹性自动扩展        |
| **监控目标** | 1000+                    | 10000+                               | 无限制              |
| **支持服务** | 社区支持                 | 专业支持 + SLA                       | 24/7 云支持         |

#### 单机版技术实现路线

| 技术领域       | 选择方案        | 替代方案       | 选择理由 |
| -------------- | --------------- | -------------- | -------- |
| **Web 框架**   | Gin             | Echo, Fiber    | 成熟生态 |
| **模板引擎**   | Templ           | html/template  | 类型安全 |
| **前端技术**   | HTMX+Tailwind   | React/Vue      | SSR      |
| **关系数据库** | SQLite          | PostgreSQL     | 零配置   |
| **时序数据库** | VictoriaMetrics | InfluxDB       | 高性能   |
| **ORM**        | Ent             | GORM           | 类型安全 |
| **消息队列**   | NATS Embedded   | Redis,RabbitMQ | 轻量级   |
| **缓存**       | Ristretto       | BigCache       | 高性能   |

### 3.3 架构决策记录 (ADR)

#### ADR-001: 选择单体架构而非微服务

**状态**: 已决定
**日期**: 2024-01-15

**背景**: 需要为小团队和个人开发者提供开箱即用的监控平台

**决策**: 采用模块化单体架构，而非微服务架构

**理由**:

- 简化部署和运维复杂度
- 降低网络通信开销
- 便于开发和调试
- 满足目标用户的规模需求

**后果**:

- 优点: 部署简单、性能更好、开发效率高
- 缺点: 水平扩展受限（通过集群版解决）

#### ADR-002: 选择嵌入式数据库

**状态**: 已决定
**日期**: 2024-01-16

**背景**: 需要零依赖的数据持久化方案

**决策**: 使用 SQLite 作为关系数据库，VictoriaMetrics 作为时序数据库

**理由**:

- SQLite: 无需配置、事务支持、成熟稳定
- VictoriaMetrics: 可嵌入、高性能、Prometheus 兼容

## 4. 核心模块

### 4.1 系统启动流程

#### 应用程序入口

```go
// Application entry point
func main() {
    // Parse command line arguments and config
    config, err := parseConfig()
    if err != nil {
        log.Fatal("Failed to parse config:", err)
    }

    // Initialize structured logger
    logger, err := initLogger(config.Log)
    if err != nil {
        log.Fatal("Failed to initialize logger:", err)
    }

    // Create application instance
    app, err := NewApplication(config, logger)
    if err != nil {
        logger.Fatal("Failed to create application", zap.Error(err))
        return
    }

    // Start application services
    if err := app.Start(); err != nil {
        logger.Fatal("Failed to start application", zap.Error(err))
        return
    }

    // Wait for shutdown signal
    app.WaitForShutdown()
}
```

#### 应用程序结构

```go
// Application represents the main application structure
type Application struct {
    // Configuration and logging
    config *Config
    logger *zap.Logger

    // Data layer components
    db    *ent.Client          // SQLite database client
    tsdb  *vm.Client           // VictoriaMetrics client
    cache *ristretto.Cache     // Memory cache
    nats  *nats.Server         // Embedded NATS server

    // Service layer components
    collectorMgr    *collector.Manager
    alertMgr        *alert.Manager
    notificationMgr *notification.Manager
    templateMgr     *template.Manager
    authMgr         *auth.Manager

    // Server components
    httpServer    *http.Server
    metricsServer *http.Server

    // Lifecycle management
    ctx      context.Context
    cancel   context.CancelFunc
    wg       sync.WaitGroup
    shutdown chan os.Signal
}

func NewApplication(config *Config, logger *zap.Logger) (*Application, error) {
    ctx, cancel := context.WithCancel(context.Background())

    app := &Application{
        config:   config,
        logger:   logger,
        ctx:      ctx,
        cancel:   cancel,
        shutdown: make(chan os.Signal, 1),
    }

    // 初始化各个组件
    if err := app.initializeComponents(); err != nil {
        return nil, err
    }

    return app, nil
}
```

#### 组件初始化顺序

```go
func (app *Application) initializeComponents() error {
    var err error

    // 1. 初始化存储层（最基础的依赖）
    if err = app.initStorage(); err != nil {
        return fmt.Errorf("storage initialization failed: %w", err)
    }

    // 2. 初始化缓存（无外部依赖）
    if err = app.initCache(); err != nil {
        return fmt.Errorf("cache initialization failed: %w", err)
    }

    // 3. 初始化消息队列（服务间通信）
    if err = app.initMessageBus(); err != nil {
        return fmt.Errorf("message bus initialization failed: %w", err)
    }

    // 4. 初始化核心服务（依赖存储和消息队列）
    if err = app.initCoreServices(); err != nil {
        return fmt.Errorf("core services initialization failed: %w", err)
    }

    // 5. 初始化HTTP服务器（最后启动，对外暴露）
    if err = app.initHTTPServer(); err != nil {
        return fmt.Errorf("HTTP server initialization failed: %w", err)
    }

    return nil
}
```

#### 优雅关闭机制

```go
func (app *Application) WaitForShutdown() {
    signal.Notify(app.shutdown, syscall.SIGINT, syscall.SIGTERM)

    <-app.shutdown
    app.logger.Info("received shutdown signal, starting graceful shutdown")

    // 创建关闭超时上下文
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(), 30*time.Second)
    defer shutdownCancel()

    // 1. 停止接受新请求
    app.logger.Info("stopping HTTP server")
    if err := app.httpServer.Shutdown(shutdownCtx); err != nil {
        app.logger.Error("HTTP server shutdown error", zap.Error(err))
    }

    // 2. 停止核心服务
    app.logger.Info("stopping core services")
    app.stopCoreServices(shutdownCtx)

    // 3. 关闭数据库连接
    app.logger.Info("closing database connections")
    if err := app.db.Close(); err != nil {
        app.logger.Error("database close error", zap.Error(err))
    }

    // 4. 等待所有goroutine完成
    done := make(chan struct{})
    go func() {
        app.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        app.logger.Info("graceful shutdown completed")
    case <-shutdownCtx.Done():
        app.logger.Warn("shutdown timeout, forcing exit")
    }
}
```

### 4.2 核心模块设计

#### 采集器框架 (Collector Framework)

#### 接口定义

```go
// Collector defines the interface for data collection
type Collector interface {
    // Basic information
    Name() string
    Type() CollectorType

    // Data collection
    Collect(ctx context.Context) ([]Metric, error)

    // Configuration and lifecycle
    ValidateConfig(config Config) error
    Start(ctx context.Context) error
    Stop() error

    // Health check
    Health() error
}

// Collection scheduler manages collector execution
type Scheduler struct {
    collectors    map[string]Collector
    cronJobs      map[string]*cron.Cron
    rateLimiter   *rate.Limiter
    workerPool    *WorkerPool
    metrics       *SchedulerMetrics
}
```

#### 数据流程

1. Scheduler 根据 cron 表达式触发采集
2. Collector 执行具体采集逻辑
3. 数据写入 VictoriaMetrics
4. 发送采集事件到 NATS

#### 告警引擎 (Alert Engine)

#### 规则定义

```go
// AlertRule defines an alert rule configuration
type AlertRule struct {
    ID          string            `json:"id" yaml:"id"`
    Name        string            `json:"name" yaml:"name"`
    Query       string            `json:"query" yaml:"query"`
    // PromQL expression
    Duration    time.Duration     `json:"duration" yaml:"duration"`
    // Hold duration
    Severity    AlertSeverity     `json:"severity" yaml:"severity"`
    Condition   AlertCondition    `json:"condition" yaml:"condition"`
    Labels      map[string]string `json:"labels" yaml:"labels"`
    Annotations map[string]string `json:"annotations" yaml:"annotations"`
    Enabled     bool              `json:"enabled" yaml:"enabled"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
}

// Alert state machine manages alert lifecycle
type AlertStateMachine struct {
    states      map[string]AlertState
    transitions []StateTransition
    rules       []TransitionRule
}
```

#### 状态转换

```text
inactive → pending → firing → resolved
    ↑        ↓       ↓        ↓
    └────────┴───────┴────────┘
```

#### 通知服务 (Notification Service)

#### 通知接口定义

```go
// Notifier defines the interface for notification delivery
type Notifier interface {
    // Basic information
    Name() string
    Type() NotifierType

    // Notification delivery
    Send(ctx context.Context, notification Notification) error

    // Configuration and validation
    ValidateConfig(config Config) error
    Test(ctx context.Context, config Config) error

    // Supported message formats
    SupportedFormats() []MessageFormat
}

// Notification router manages routing rules
type NotificationRouter struct {
    routes         []Route
    notifiers      map[string]Notifier
    rateLimiter    *RateLimiter
    retryManager   *RetryManager
    metrics        *RouterMetrics
}
```

#### 路由策略

- **标签匹配**: 基于标签匹配
- **正则支持**: 支持正则表达式
- **时间过滤**: 时间段过滤
- **分组管理**: 接收人分组

## 5. 数据模型

### 5.1 时序数据模型

```go
// Metric represents a time series data point
type Metric struct {
    Name      string            `json:"name"`
    Labels    map[string]string `json:"labels"`
    Value     float64           `json:"value"`
    Timestamp int64             `json:"timestamp"`
}
```

### 5.2 关系数据模型 (Ent Schema)

// Monitor - 监控配置
type Monitor struct {
ent.Schema
}

func (Monitor) Fields() []ent.Field {
return []ent.Field{
field.String("name").NotEmpty(),
field.String("type"),
field.JSON("config", map[string]interface{}{}),
field.String("interval"),
field.Bool("enabled").Default(true),
}
}

// AlertRule - 告警规则
type AlertRule struct {
ent.Schema
}

func (AlertRule) Fields() []ent.Field {
return []ent.Field{
field.String("name").NotEmpty(),
field.String("query"),
field.String("duration"),
field.Enum("severity").Values("info", "warning", "critical"),
field.JSON("labels", map[string]string{}),
}
}

## 6. 安全架构

### 6.1 认证与授权

#### 安全设计原则

- **最小权限原则**: 默认最小权限，按需授权
- **分层防护**: 网络、应用、数据多层防护
- **零信任架构**: 所有访问都需认证和授权
- **安全默认**: 安全配置作为默认选项
- **审计追踪**: 完整的操作日志和审计踪迹

#### 主进程结构

```go
type Application struct {
    // Core Components
    server      *gin.Engine
    scheduler   *scheduler.Manager
    alertMgr    *alert.Manager
    notifier    *notification.Manager
    templateMgr *template.Manager
    agentMgr    *agent.Manager
    cmdExecutor *command.Executor
    terminalSvc *terminal.Service
    pushGateway *push.Gateway

    // Storage
    db         *ent.Client        // SQLite
    tsdb       *vm.Client         // VictoriaMetrics
    cache      *ristretto.Cache   // Memory Cache

    // Message Bus
    nats       *nats.Server       // Embedded NATS

    // Security
    authMgr    *auth.Manager
    rbac       *rbac.Manager
    encryption *encrypt.Manager

    // Lifecycle
    ctx        context.Context
    cancel     context.CancelFunc
    wg         sync.WaitGroup
}
```

#### 启动流程

```text
1. 初始化配置 → 解析命令行参数和配置文件
2. 初始化存储 → SQLite + VictoriaMetrics + Cache
3. 启动NATS  → 内嵌消息总线
4. 启动核心服务 → 采集器、告警引擎、通知服务
5. 启动HTTP服务器 → Web UI + API
6. 注册信号处理 → 优雅关闭
```

#### 存储分层设计

```text
┌─────────────────────────────────────────────┐
│              Application Layer              │
├─────────────────────────────────────────────┤
│                                             │
│ ┌─────────────┐ ┌─────────────┐ ┌─────────┐ │
│ │  Metadata   │ │ Time Series │ │  Cache  │ │
│ │   (SQLite)  │ │ (Victoria)  │ │ (Memory)│ │
│ └─────────────┘ └─────────────┘ └─────────┘ │
│       │               │             │       │
│ ┌─────────────┐ ┌─────────────┐ ┌─────────┐ │
│ │ Config      │ │ Metrics     │ │ Sessions│ │
│ │ Users       │ │ Alerts      │ │ Queries │ │
│ │ Rules       │ │ Events      │ │ Templates│ │
│ │ Templates   │ │             │ │         │ │
│ └─────────────┘ └─────────────┘ └─────────┘ │
├─────────────────────────────────────────────┤
│                File System                  │
│  /data/watchdog/                           │
│  ├── config/                               │
│  ├── db/                                   │
│  ├── metrics/                              │
│  └── logs/                                 │
└─────────────────────────────────────────────┘
```

#### 数据目录结构

```text
/data/watchdog/
├── config/
│   ├── watchdog.yaml        # 主配置文件
│   ├── monitors/             # 监控配置
│   ├── rules/                # 告警规则
│   └── templates/            # 模板文件
├── db/
│   └── watchdog.db          # SQLite数据库
├── metrics/
│   └── victoria-metrics/    # 时序数据
├── logs/
│   ├── watchdog.log         # 应用日志
│   ├── access.log           # 访问日志
│   └── audit.log            # 审计日志
└── tmp/
    ├── scripts/             # 临时脚本
    └── exports/             # 导出文件
```

#### 端口规划

```yaml
ports:
  main:
    web: 8080 # Web UI + API
    metrics: 8081 # Prometheus Metrics
    health: 8082 # Health Check

  embedded:
    victoria: 8428 # VictoriaMetrics (内部)
    nats: 4222 # NATS (内部)

  external:
    webhook: 8080/api/v1/webhook # Webhook接收
    push: 8080/api/v1/push # Push数据接收
```

#### 网络安全

```text
┌─────────────────────────────────────────┐
│             Load Balancer               │
│           (Optional Reverse Proxy)      │
└─────────────────┬───────────────────────┘
                  │ HTTPS/TLS 1.3
┌─────────────────▼───────────────────────┐
│             Watchdog Server             │
│                                         │
│ ┌─────────────┐ ┌─────────────────────┐ │
│ │   Auth      │ │    Rate Limiting    │ │
│ │ Middleware  │ │    • API: 1000/min  │ │
│ │             │ │    • Web: 100/min   │ │
│ │ • JWT       │ │    • Push: 10000/min│ │
│ │ • API Key   │ │                     │ │
│ │ • Session   │ │                     │ │
│ └─────────────┘ └─────────────────────┘ │
└─────────────────────────────────────────┘
```

#### 分布式架构

```yaml
services:
  watchdog:
    replicas: 3
    resources:
      cpu: 2
      memory: 2Gi

  victoria-metrics:
    replicas: 1
    storage: 100Gi

  postgresql:
    replicas: 2
    storage: 50Gi

  nats:
    replicas: 3
    mode: cluster

  redis:
    replicas: 3
    mode: cluster
```

## 8. 安全架构

### 8.1 单机版安全设计

#### 认证机制

```go
type AuthManager struct {
    // 本地用户存储
    userStore   *UserStore

    // 会话管理
    sessionStore *SessionStore

    // JWT配置
    jwtSecret   []byte
    jwtExpiry   time.Duration

    // API Key管理
    apiKeyStore *APIKeyStore
}

// 支持的认证方式
type AuthMethod int
const (
    AuthMethodLocal AuthMethod = iota  // 用户名密码
    AuthMethodJWT                      // JWT Token
    AuthMethodAPIKey                   // API密钥
    AuthMethodSession                  // 会话Cookie
)
```

#### 密码安全

- **密码策略**: 最少 8 位，包含大小写字母、数字和特殊字符
- **密码存储**: bcrypt with cost 12
- **密码重置**: 安全问题 + 邮件验证
- **账户锁定**: 5 次失败后锁定 30 分钟

### 8.2 授权模型

#### 单机版 RBAC

```yaml
roles:
  admin:
    permissions:
      - monitors:*
      - alerts:*
      - notifications:*
      - users:*
      - config:*
      - system:*

  operator:
    permissions:
      - monitors:read
      - monitors:create
      - monitors:update
      - alerts:*
      - notifications:read
      - notifications:create

  viewer:
    permissions:
      - monitors:read
      - alerts:read
      - notifications:read
      - dashboard:read

resources:
  monitors:
    actions: [create, read, update, delete, execute]
  alerts:
    actions: [create, read, update, delete, acknowledge, silence]
  notifications:
    actions: [create, read, update, delete, send, test]
  users:
    actions: [create, read, update, delete, reset_password]
  config:
    actions: [read, update, backup, restore]
  system:
    actions: [read, restart, shutdown, logs]
```

#### 权限检查中间件

```go
func RequirePermission(resource, action string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user := GetCurrentUser(c)
        if user == nil {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }

        if !user.HasPermission(resource, action) {
            c.JSON(403, gin.H{"error": "forbidden"})
            c.Abort()
            return
        }

        c.Next()
    }
}
```

### 8.3 数据安全

#### 传输安全

- **TLS 1.3**: 强制 HTTPS，禁用低版本 TLS
- **HSTS**: HTTP 严格传输安全
- **证书管理**: 自动申请 Let's Encrypt 证书
- **内部通信**: 组件间 TLS 加密

#### 存储安全

```go
type EncryptionManager struct {
    key    []byte    // AES-256 主密钥
    keyDerivation *KeyDerivation  // 密钥派生
}

// 敏感字段加密
type EncryptedField struct {
    Value     []byte `json:"value"`      // 加密数据
    Nonce     []byte `json:"nonce"`      // 随机数
    Algorithm string `json:"algorithm"`  // 加密算法
}

// 需要加密的字段
var EncryptedFields = []string{
    "password",
    "api_key",
    "webhook_secret",
    "smtp_password",
    "telegram_token",
}
```

#### 审计日志

```go
type AuditLog struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    UserID    string    `json:"user_id"`
    UserIP    string    `json:"user_ip"`
    Action    string    `json:"action"`
    Resource  string    `json:"resource"`
    Details   string    `json:"details"`
    Success   bool      `json:"success"`
}

// 审计事件类型
const (
    AuditLogin         = "auth.login"
    AuditLogout        = "auth.logout"
    AuditMonitorCreate = "monitor.create"
    AuditMonitorUpdate = "monitor.update"
    AuditMonitorDelete = "monitor.delete"
    AuditRuleCreate    = "rule.create"
    AuditRuleUpdate    = "rule.update"
    AuditRuleDelete    = "rule.delete"
    AuditConfigUpdate  = "config.update"
)
```

### 8.4 网络安全

#### 防护措施

```yaml
security:
  rate_limiting:
    global: 10000/hour
    per_ip: 1000/hour
    api: 100/minute
    login: 5/minute

  cors:
    enabled: true
    origins: ["https://your-domain.com"]
    methods: ["GET", "POST", "PUT", "DELETE"]
    headers: ["Authorization", "Content-Type"]

  headers:
    csp: "default-src 'self'; script-src 'self' 'unsafe-inline'"
    hsts: "max-age=31536000; includeSubDomains"
    x_frame_options: "DENY"
    x_content_type_options: "nosniff"
```

#### 入侵检测

```go
type SecurityMonitor struct {
    failedLogins   map[string]int      // IP -> 失败次数
    suspiciousIPs  map[string]time.Time // IP -> 最后可疑活动时间
    rateLimiter    *RateLimiter
}

// 检测规则
var SecurityRules = []SecurityRule{
    {
        Name: "BruteForceLogin",
        Condition: "failed_login_count > 5 in 10m",
        Action: "block_ip",
        Duration: time.Hour,
    },
    {
        Name: "SuspiciousUserAgent",
        Condition: "user_agent matches bot_patterns",
        Action: "require_captcha",
    },
    {
        Name: "AbnormalAPIUsage",
        Condition: "api_calls > 1000 in 1m",
        Action: "rate_limit",
    },
}
```

## 9. 性能设计

### 9.1 单机版性能目标

#### 核心指标

```yaml
performance_targets:
  # 数据采集
  collection:
    throughput: 1000 metrics/s # 采集吞吐量
    latency_p99: 5s # 采集延迟
    concurrent_jobs: 100 # 并发采集任务

  # 查询性能
  query:
    latency_p95: 500ms # 查询响应时间
    latency_p99: 1s
    concurrent_queries: 50 # 并发查询数

  # 告警处理
  alerting:
    evaluation_interval: 10s # 告警评估间隔
    rule_capacity: 1000 # 告警规则数量
    notification_latency: 30s # 通知延迟

  # 系统资源
  resource:
    cpu_usage: 50% # CPU使用率
    memory_usage: 1GB # 内存使用
    disk_usage: 10GB/month # 磁盘增长

  # 并发能力
  concurrency:
    web_users: 20 # 并发Web用户
    api_clients: 100 # 并发API客户端
    websocket_connections: 50 # WebSocket连接
```

### 9.2 架构优化策略

#### 数据采集优化

```go
type CollectionOptimizer struct {
    // 批量处理
    batchSize     int           // 批量大小
    batchTimeout  time.Duration // 批量超时

    // 连接池
    httpPool      *HTTPPool     // HTTP连接池

    // 限流器
    rateLimiter   *RateLimiter  // 全局限流

    // 缓存
    dnsCache      *DNSCache     // DNS缓存
    resultCache   *ResultCache  // 结果缓存
}

// 优化配置
type OptimizationConfig struct {
    // 批量写入
    BatchWrite struct {
        Size    int           `yaml:"size"`     // 1000条
        Timeout time.Duration `yaml:"timeout"`  // 5秒
    }

    // 连接池
    HTTPPool struct {
        MaxConns        int           `yaml:"max_conns"`         // 100
        MaxIdleConns    int           `yaml:"max_idle_conns"`    // 50
        IdleTimeout     time.Duration `yaml:"idle_timeout"`      // 30秒
        RequestTimeout  time.Duration `yaml:"request_timeout"`   // 30秒
    }

    // 缓存
    Cache struct {
        QueryTTL    time.Duration `yaml:"query_ttl"`     // 5分钟
        DNSTTL      time.Duration `yaml:"dns_ttl"`       // 1小时
        MaxSize     int           `yaml:"max_size"`      // 100MB
    }
}
```

#### 查询性能优化

```go
type QueryOptimizer struct {
    // 查询缓存
    cache         *QueryCache

    // 索引管理
    indexManager  *IndexManager

    // 查询重写
    rewriter      *QueryRewriter
}

// 查询优化策略
var QueryOptimizations = []Optimization{
    {
        Name: "TimeRangeOptimization",
        Apply: func(query *Query) *Query {
            // 自动调整时间范围
            if query.Range > 7*24*time.Hour {
                query.Step = time.Hour // 长时间范围降低精度
            }
            return query
        },
    },
    {
        Name: "MetricFiltering",
        Apply: func(query *Query) *Query {
            // 提前过滤不必要的指标
            return query.AddFilter("__name__", query.MetricName)
        },
    },
}
```

#### 内存管理优化

```go
type MemoryManager struct {
    // 对象池
    metricPool    sync.Pool  // Metric对象池
    requestPool   sync.Pool  // Request对象池

    // 内存监控
    memStats      *MemStats
    gcTrigger     *GCTrigger
}

// 内存优化配置
type MemoryConfig struct {
    // GC调优
    GCTarget     int     `yaml:"gc_target"`      // 100 (GOGC)
    MaxMemory    string  `yaml:"max_memory"`     // "1GB"

    // 对象池
    PoolEnabled  bool    `yaml:"pool_enabled"`   // true
    PoolMaxSize  int     `yaml:"pool_max_size"`  // 1000

    // 缓存策略
    CachePolicy  string  `yaml:"cache_policy"`   // "lru"
    CacheSize    string  `yaml:"cache_size"`     // "100MB"
}
```

### 9.3 监控与调优

#### 性能监控指标

```yaml
monitoring:
  application:
    - watchdog_http_requests_duration_seconds
    - watchdog_collection_duration_seconds
    - watchdog_alert_evaluation_duration_seconds
    - watchdog_notification_duration_seconds

  system:
    - process_cpu_seconds_total
    - process_resident_memory_bytes
    - go_memstats_alloc_bytes
    - go_memstats_gc_duration_seconds

  business:
    - watchdog_active_monitors_total
    - watchdog_active_alerts_total
    - watchdog_metrics_ingested_total
    - watchdog_notifications_sent_total
```

#### 自动调优机制

```go
type AutoTuner struct {
    // 性能采样
    sampler       *PerformanceSampler

    // 调优策略
    strategies    []TuningStrategy

    // 配置管理
    configManager *ConfigManager
}

// 调优策略
var TuningStrategies = []TuningStrategy{
    {
        Name: "BatchSizeAdjustment",
        Trigger: "avg_write_latency > 1s",
        Action: "decrease_batch_size",
    },
    {
        Name: "CacheSizeAdjustment",
        Trigger: "cache_hit_ratio < 80%",
        Action: "increase_cache_size",
    },
    {
        Name: "GCTuning",
        Trigger: "gc_pause_time > 100ms",
        Action: "adjust_gc_target",
    },
}
```

## 10. 可观测性设计

### 10.1 指标体系

#### 系统指标暴露

**Prometheus 格式指标** (`/metrics`)

```yaml
metrics:
  # 应用指标
  application:
    - watchdog_info{version,build_time,go_version}
    - watchdog_uptime_seconds
    - watchdog_config_last_reload_timestamp

  # 采集指标
  collection:
    - watchdog_collectors_total{type,status}
    - watchdog_collection_duration_seconds{collector,status}
    - watchdog_collection_errors_total{collector,error_type}
    - watchdog_metrics_ingested_total{collector}

  # 告警指标
  alerting:
    - watchdog_alert_rules_total{status}
    - watchdog_alerts_active{rule,severity}
    - watchdog_alert_evaluation_duration_seconds{rule}
    - watchdog_alert_evaluation_failures_total{rule}

  # 通知指标
  notification:
    - watchdog_notifications_sent_total{notifier,status}
    - watchdog_notification_duration_seconds{notifier}
    - watchdog_notification_errors_total{notifier,error_type}

  # HTTP指标
  http:
    - watchdog_http_requests_total{method,path,status}
    - watchdog_http_request_duration_seconds{method,path}
    - watchdog_http_request_size_bytes{method,path}
    - watchdog_http_response_size_bytes{method,path}

  # 数据库指标
  database:
    - watchdog_db_connections_active
    - watchdog_db_connections_idle
    - watchdog_db_query_duration_seconds{query_type}
    - watchdog_db_size_bytes{database}

  # 缓存指标
  cache:
    - watchdog_cache_hits_total{cache_name}
    - watchdog_cache_misses_total{cache_name}
    - watchdog_cache_size_bytes{cache_name}
    - watchdog_cache_evictions_total{cache_name}
```

#### 指标采集实现

```go
type MetricsCollector struct {
    // Prometheus注册器
    registry *prometheus.Registry

    // 业务指标
    collectorsTotal     *prometheus.CounterVec
    collectionDuration  *prometheus.HistogramVec
    alertsActive        *prometheus.GaugeVec
    notificationsSent   *prometheus.CounterVec

    // 系统指标
    httpRequests        *prometheus.CounterVec
    httpDuration        *prometheus.HistogramVec
    dbConnections       *prometheus.GaugeVec
    cacheHitRatio       *prometheus.GaugeVec
}

// 指标更新
func (m *MetricsCollector) RecordCollection(
    collector string, duration time.Duration, success bool) {
    status := "success"
    if !success {
        status = "error"
    }

    m.collectorsTotal.WithLabelValues(collector, status).Inc()
    m.collectionDuration.WithLabelValues(collector, status).Observe(duration.Seconds())
}
```

### 10.2 日志系统

#### 结构化日志

```go
type Logger struct {
    *zap.Logger
    fields []zap.Field
}

// 日志级别和格式
type LogConfig struct {
    Level       string `yaml:"level"`        // debug, info, warn, error
    Format      string `yaml:"format"`       // json, console
    Output      string `yaml:"output"`       // stdout, file
    File        string `yaml:"file"`         // 日志文件路径
    MaxSize     int    `yaml:"max_size"`     // MB
    MaxBackups  int    `yaml:"max_backups"`  // 保留文件数
    MaxAge      int    `yaml:"max_age"`      // 保留天数
    Compress    bool   `yaml:"compress"`     // 压缩
}

// 结构化日志示例
logger.Info("collector started",
    zap.String("collector_id", id),
    zap.String("collector_type", collectorType),
    zap.Duration("interval", interval),
    zap.Int("timeout_seconds", timeout),
)

logger.Error("collection failed",
    zap.String("collector_id", id),
    zap.String("target", target),
    zap.Error(err),
    zap.Duration("duration", duration),
)
```

#### 日志分类

````yaml
log_categories:
  # 应用日志
  application:
    file: "watchdog.log"
    level: "info"
    format: "json"

  # 访问日志
  access:
    file: "access.log"
    format: "combined"
    fields: ["timestamp", "method", "path", "status", "duration", "ip", "user_agent"]

  # 审计日志
  audit:
    file: "audit.log"
    level: "info"
    format: "json"
    fields: ["timestamp", "user_id", "action", "resource", "ip", "user_agent", "success"]

  # 错误日志
  error:
    file: "error.log"
    level: "error"
    format: "json"
    extra_context: true

  # 调试日志
  debug:
    file: "debug.log"
    level: "debug"
    format: "console"
    enabled: false  # 生产环境禁用

## 13. 实施计划

### 13.1 开发阶段划分（与 PRD 一致）

#### M1: 核心框架 (第 1-3 月)

**阶段目标**: 建立技术基础，验证核心架构

**核心交付**：
- ✅ **基础监控能力**: HTTP/API 监控、系统资源监控、基础指标采集
- ✅ **告警系统**: 基础告警引擎、邮件通知、阈值告警
- ✅ **用户界面**: 简化 Web UI、RESTful API、基础仪表盘
- ✅ **部署方案**: Docker 单机部署、基础文档、快速开始指南

**验收标准**：
- 单机部署时间 ≤ 5 分钟
- 支持 50+ 监控目标
- 基础告警功能完全可用
- 系统资源占用<100MB

#### M2: 模板生态 (第 4-6 月)

**阶段目标**: 建立差异化优势，丰富监控场景

**核心交付**：
- ✅ **垂直模板库**: 金融、天气、系统监控等 15+ 行业模板
- ✅ **智能 Agent**: 轻量级 Agent、自动化任务调度、远程执行
- ✅ **高级功能**: 智能告警策略、数据可视化、趋势分析
- ✅ **用户体验**: 完整文档、最佳实践、社区支持

**验收标准**：
- 15+ 垂直场景模板可用
- 支持 1000+ 监控目标
- 用户配置时间 ≤ 10 分钟
- 模板部署成功率>95%

#### M3: 企业就绪 (第 7-9 月)

**阶段目标**: 达到生产就绪，启动商业化

**核心交付**：
- ✅ **企业级功能**: 多租户支持、RBAC 权限、SSO 集成
- ✅ **高可用架构**: 集群部署、数据备份恢复、故障转移
- ✅ **自动化运维**: 指令执行、协作终端、运维模板体系
- ✅ **集成能力**: 完整 API 文档、Webhook 集成、第三方平台支持
- ✅ **商业版本**: SaaS 服务 Beta 版、定价策略、客户支持

**验收标准**：
- 系统可用性 ≥ 99.9%
- 通过企业级安全认证
- 获得首批 10+ 付费客户
- 月收入达到 $10K+

#### M4: 规模化增长 (第 10-12 月)

**阶段目标**: 建立市场领导地位，实现规模化盈利

**核心交付**：
- ✅ **AI 驱动的智能监控**: 异常检测、智能降噪、预测分析
- ✅ **垂直行业解决方案**: 金融、制造业、IoT 等专业化解决方案
- ✅ **生态伙伴和集成平台**: 插件市场、合作伙伴生态
- ✅ **国际化和多区域部署**: 全球服务能力、本地化支持

**发展指标**：
- 开源用户: 20000+
- 付费用户: 2000+
- 企业客户: 200+
- 年收入: $5M+

### 13.2 技术风险评估（与 PRD 一致）

| 风险项 | 风险等级 | 影响 | 应对策略 |
|--------|---------|-----|---------|
| **时序数据库性能** | 中 | 用户体验下降 | VictoriaMetrics 备选，分层存储策略 |
| **前端技术栈** | 低 | 开发效率 | HTMX 渐进式采用，React 备选方案 |
| **单机性能极限** | 中 | 扩展性限制 | 分布式架构预案，性能优化 |
| **Go 生态依赖** | 低 | 维护成本 | 减少外部依赖，核心组件自研 |
| **模板系统复杂度** | 中 | 用户采用 | 分阶段实现，从简单模板开始 |
| **自动化运维安全** | 高 | 安全风险 | 严格权限控制，安全审计，砂箱机制 |

### 13.3 质量保证策略

#### 测试策略
```yaml
testing:
  unit_tests:
    coverage_target: 80%
    critical_paths: 90%

  integration_tests:
    database_layer: true
    api_endpoints: true
    collector_framework: true

  performance_tests:
    load_testing: true
    stress_testing: true
    benchmark_testing: true

  security_tests:
    auth_testing: true
    input_validation: true
    vulnerability_scanning: true
````

#### 代码质量

- **静态分析**: golangci-lint, gosec
- **代码审查**: 强制 PR 审查
- **文档**: 完整的 API 文档和架构文档
- **CI/CD**: 自动化测试和部署

## 14. 总结与展望

### 14.1 架构优势

1. **简单易用**: 单机部署，零配置依赖
2. **高性能**: 优化的数据流和存储架构
3. **可扩展**: 模块化设计，插件体系
4. **生产就绪**: 完整的监控、日志、备份机制
5. **成本效益**: 开源免费，维护成本低

### 14.2 技术创新点

1. **嵌入式监控**: VictoriaMetrics + SQLite 嵌入式集成
2. **模板驱动**: 参数化监控模板，快速复制
3. **轻量级 Agent**: 单二进制，最小资源占用
4. **现代化前端**: HTMX + TailwindCSS，减少复杂性
5. **智能告警**: 多维度告警策略和生命周期管理

### 14.3 未来发展方向

#### 短期目标 (6 个月)

- 完成单机版 MVP
- 建立用户社区
- 收集用户反馈
- 性能和稳定性优化

#### 中期目标 (1 年)

- 推出集群版本
- 建立插件生态
- 扩展监控模板库
- AI 驱动的异常检测

#### 长期愿景 (2-3 年)

- 成为小团队监控首选
- 建立商业化产品线
- 支持更多垂直场景
- 国际化和合规认证

### 14.4 成功指标

**技术指标**:

- 单机支持 1000+监控目标
- 95%查询响应时间<1 秒
- 99.9%系统可用性
- 内存占用<1GB

**业务指标**:

- GitHub Stars 1000+
- Docker 下载量 10000+
- 活跃用户 500+
- 社区贡献者 20+

通过这个完整的架构设计，Watchdog 将成为一个真正开箱即用、面向小团队的监控平台，在简单性和功能完整性之间找到最佳平衡点。

#### 健康检查端点

```go
// 健康检查接口
type HealthChecker interface {
    Name() string
    Check(ctx context.Context) error
}

// 健康检查实现
type HealthManager struct {
    checkers []HealthChecker
    cache    *HealthCache
}

// 健康检查端点
func (h *HealthManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
    defer cancel()

    result := &HealthResult{
        Status: "healthy",
        Timestamp: time.Now(),
        Checks: make(map[string]CheckResult),
    }

    for _, checker := range h.checkers {
        checkResult := CheckResult{
            Name: checker.Name(),
            Status: "healthy",
        }

        start := time.Now()
        if err := checker.Check(ctx); err != nil {
            checkResult.Status = "unhealthy"
            checkResult.Error = err.Error()
            result.Status = "unhealthy"
        }
        checkResult.Duration = time.Since(start)

        result.Checks[checker.Name()] = checkResult
    }

    w.Header().Set("Content-Type", "application/json")
    if result.Status == "unhealthy" {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    json.NewEncoder(w).Encode(result)
}
```

#### 健康检查项目

```yaml
health_checks:
  - name: "database"
    type: "sqlite"
    query: "SELECT 1"
    timeout: "5s"

  - name: "victoria_metrics"
    type: "http"
    url: "http://localhost:8428/api/v1/query?query=up"
    timeout: "10s"

  - name: "nats"
    type: "nats"
    subject: "health.check"
    timeout: "5s"

  - name: "disk_space"
    type: "disk"
    path: "/data"
    threshold: "90%"

  - name: "memory_usage"
    type: "memory"
    threshold: "80%"
```

### 10.4 分布式追踪

#### OpenTelemetry 集成

```go
type TracingConfig struct {
    Enabled     bool   `yaml:"enabled"`
    ServiceName string `yaml:"service_name"`
    Endpoint    string `yaml:"endpoint"`
    Sampler     string `yaml:"sampler"`      // always, never, ratio
    SampleRate  float64 `yaml:"sample_rate"` // 0.1 = 10%
}

// 追踪初始化
func InitTracing(config TracingConfig) error {
    if !config.Enabled {
        return nil
    }

    // 创建追踪提供者
    tp, err := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(otlptracegrpc.New(
            context.Background(),
            otlptracegrpc.WithEndpoint(config.Endpoint),
        )),
        sdktrace.WithSampler(createSampler(config)),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(config.ServiceName),
            semconv.ServiceVersionKey.String(version.Version),
        )),
    )

    otel.SetTracerProvider(tp)
    return nil
}

// 追踪中间件
func TracingMiddleware() gin.HandlerFunc {
    return otelgin.Middleware("watchdog")
}
```

#### 关键路径追踪

```go
// 采集链路追踪
func (c *Collector) Collect(ctx context.Context) error {
    ctx, span := otel.Tracer("collector").Start(ctx, "collect",
        trace.WithAttributes(
            attribute.String("collector.name", c.Name()),
            attribute.String("collector.type", c.Type()),
        ),
    )
    defer span.End()

    // 执行采集逻辑
    metrics, err := c.doCollect(ctx)
    if err != nil {
        span.SetStatus(codes.Error, err.Error())
        return err
    }

    span.SetAttributes(
        attribute.Int("metrics.count", len(metrics)),
    )

    // 写入数据库
    return c.store.Write(ctx, metrics)
}
```

## 11. 扩展性设计

### 11.1 插件系统架构

#### 插件框架设计

```go
// 插件接口定义
type Plugin interface {
    // 基础信息
    Name() string
    Version() string
    Description() string
    Author() string

    // 生命周期
    Init(ctx context.Context, config Config) error
    Start() error
    Stop() error
    Health() error

    // 配置
    Schema() ConfigSchema
    Validate(config Config) error
}

// 插件管理器
type PluginManager struct {
    plugins     map[string]Plugin
    registry    *PluginRegistry
    loader      *PluginLoader
    configMgr   *ConfigManager
}

// 插件注册表
type PluginRegistry struct {
    collectors  map[string]CollectorPlugin
    notifiers   map[string]NotifierPlugin
    auth        map[string]AuthPlugin
    storage     map[string]StoragePlugin
    middleware  map[string]MiddlewarePlugin
}
```

#### 插件加载机制

```go
// 插件配置
type PluginConfig struct {
    Name        string                 `yaml:"name"`
    Type        string                 `yaml:"type"`
    Version     string                 `yaml:"version"`
    Enabled     bool                   `yaml:"enabled"`
    Config      map[string]interface{} `yaml:"config"`
    Dependencies []string              `yaml:"dependencies"`
}

// 插件加载器
type PluginLoader struct {
    pluginDir   string
    symRegistry map[string]Plugin  // Go插件符号表
}

// 加载插件
func (l *PluginLoader) Load(config PluginConfig) (Plugin, error) {
    switch config.Type {
    case "builtin":
        return l.loadBuiltin(config.Name)
    case "go-plugin":
        return l.loadGoPlugin(config)
    case "wasm":
        return l.loadWasmPlugin(config)
    default:
        return nil, fmt.Errorf("unsupported plugin type: %s", config.Type)
    }
}
```

### 11.2 扩展点详细设计

#### Collector Plugin

```go
// 采集器插件接口
type CollectorPlugin interface {
    Plugin

    // 采集能力
    Collect(ctx context.Context, target Target) ([]Metric, error)

    // 配置验证
    ValidateTarget(target Target) error

    // 支持的指标类型
    SupportedMetrics() []MetricType
}

// 自定义Redis采集器示例
type RedisCollector struct {
    client *redis.Client
    config RedisConfig
}

func (r *RedisCollector) Collect(ctx context.Context, target Target) (
    []Metric, error) {
    info, err := r.client.Info(ctx).Result()
    if err != nil {
        return nil, err
    }

    metrics := []Metric{
        {
            Name: "redis_connected_clients",
            Value: parseInfo(info, "connected_clients"),
            Labels: map[string]string{
                "instance": target.Address,
                "db": target.Database,
            },
            Timestamp: time.Now(),
        },
        // 更多指标...
    }

    return metrics, nil
}
```

#### Notifier Plugin

```go
// 通知器插件接口
type NotifierPlugin interface {
    Plugin

    // 发送通知
    Send(ctx context.Context, notification Notification) error

    // 测试连接
    Test(ctx context.Context, config Config) error

    // 支持的消息格式
    SupportedFormats() []MessageFormat
}

// 自定义飞书通知器示例
type FeishuNotifier struct {
    webhook string
    secret  string
    client  *http.Client
}

func (f *FeishuNotifier) Send(ctx context.Context,
    notification Notification) error {
    message := f.formatMessage(notification)

    payload := map[string]interface{}{
        "msg_type": "text",
        "content": map[string]string{
            "text": message,
        },
    }

    // 签名验证
    if f.secret != "" {
        payload["timestamp"] = time.Now().Unix()
        payload["sign"] = f.generateSign(payload)
    }

    return f.sendWebhook(ctx, payload)
}
```

#### Auth Plugin

```go
// 认证插件接口
type AuthPlugin interface {
    Plugin

    // 认证验证
    Authenticate(ctx context.Context, credentials Credentials) (*User, error)

    // 用户信息
    GetUser(ctx context.Context, userID string) (*User, error)

    // 权限检查
    Authorize(ctx context.Context, user *User, resource, action string) bool
}

// LDAP认证插件示例
type LDAPAuthPlugin struct {
    server   string
    baseDN   string
    bindDN   string
    bindPass string
}

func (l *LDAPAuthPlugin) Authenticate(ctx context.Context,
    creds Credentials) (*User, error) {
    conn, err := ldap.DialURL(l.server)
    if err != nil {
        return nil, err
    }
    defer conn.Close()

    // 绑定用户
    userDN := fmt.Sprintf("uid=%s,%s", creds.Username, l.baseDN)
    err = conn.Bind(userDN, creds.Password)
    if err != nil {
        return nil, fmt.Errorf("authentication failed: %w", err)
    }

    // 获取用户信息
    return l.getUserInfo(conn, userDN)
}
```

### 11.3 插件开发工具链

#### 插件脚手架

```bash
# 创建插件项目
watchdog plugin init --type=collector --name=mysql

# 项目结构
mysql-collector/
├── plugin.yaml           # 插件元数据
├── main.go               # 插件入口
├── collector.go          # 采集器实现
├── config.go             # 配置定义
├── config_schema.json    # 配置模式
├── README.md            # 文档
└── examples/            # 示例配置
    └── mysql.yaml
```

#### 插件元数据

```yaml
# plugin.yaml
name: "mysql-collector"
version: "1.0.0"
type: "collector"
description: "MySQL database metrics collector"
author: "Watchdog Team"
license: "MIT"

api_version: "v1"
engine_version: ">= 1.0.0"

dependencies:
  - "database/sql"
  - "github.com/go-sql-driver/mysql"

config_schema: "config_schema.json"

supported_metrics:
  - "mysql_connections_current"
  - "mysql_queries_total"
  - "mysql_slow_queries_total"
  - "mysql_innodb_buffer_pool_size"

tags:
  - "database"
  - "mysql"
  - "performance"
```

#### 插件测试框架

```go
// 插件测试工具
type PluginTester struct {
    plugin   Plugin
    testData map[string]interface{}
}

// 标准测试用例
func TestPlugin(t *testing.T, plugin Plugin) {
    // 测试基础接口
    assert.NotEmpty(t, plugin.Name())
    assert.NotEmpty(t, plugin.Version())

    // 测试生命周期
    ctx := context.Background()
    err := plugin.Init(ctx, testConfig)
    assert.NoError(t, err)

    err = plugin.Start()
    assert.NoError(t, err)

    err = plugin.Health()
    assert.NoError(t, err)

    err = plugin.Stop()
    assert.NoError(t, err)
}

// 采集器专用测试
func TestCollectorPlugin(t *testing.T, collector CollectorPlugin) {
    TestPlugin(t, collector)

    // 测试采集功能
    ctx := context.Background()
    target := Target{
        Address: "localhost:3306",
        Database: "test",
    }

    metrics, err := collector.Collect(ctx, target)
    assert.NoError(t, err)
    assert.NotEmpty(t, metrics)

    // 验证指标格式
    for _, metric := range metrics {
        assert.NotEmpty(t, metric.Name)
        assert.NotNil(t, metric.Value)
        assert.NotZero(t, metric.Timestamp)
    }
}
```

### 11.4 插件市场与分发

#### 插件注册表

```yaml
# ~/.watchdog/registry.yaml
registries:
  official:
    url: "https://registry.watchdog.telepair.online"
    auth: false

  enterprise:
    url: "https://enterprise-registry.example.com"
    auth: true
    token: "${WATCHDOG_REGISTRY_TOKEN}"

  local:
    type: "file"
    path: "/opt/watchdog/plugins"
```

#### 插件安装工具

```bash
# 搜索插件
watchdog plugin search mysql

# 安装插件
watchdog plugin install mysql-collector@1.0.0

# 列出已安装插件
watchdog plugin list

# 更新插件
watchdog plugin update mysql-collector

# 卸载插件
watchdog plugin uninstall mysql-collector

# 插件信息
watchdog plugin info mysql-collector
```

## 12. 容灾与高可用设计

### 12.1 单机版故障处理

#### 故障分类与处理策略

```go
// 故障类型定义
type FailureType int
const (
    FailureDatabase FailureType = iota  // 数据库故障
    FailureStorage                      // 存储故障
    FailureNetwork                      // 网络故障
    FailureMemory                       // 内存不足
    FailureDisk                         // 磁盘故障
    FailureCollector                    // 采集器故障
)

// 故障处理器
type FailureHandler struct {
    handlers map[FailureType]FailureStrategy
    circuit  *CircuitBreaker
    fallback *FallbackManager
}

// 故障处理策略
type FailureStrategy interface {
    Handle(ctx context.Context, failure Failure) error
    Recover(ctx context.Context) error
    CanRecover() bool
}
```

#### 具体故障场景处理

##### 数据库故障处理

```go
type DatabaseFailureHandler struct {
    backup    *BackupManager
    cache     *MemoryCache
    readonly  *ReadOnlyMode
}

func (h *DatabaseFailureHandler) Handle(ctx context.Context,
    failure Failure) error {
    // 1. 检测故障类型
    if failure.Type == FailureDatabase {
        // 2. 激活只读模式
        h.readonly.Enable()

        // 3. 使用内存缓存
        h.cache.EnablePersistentMode()

        // 4. 尝试从备份恢复
        go h.attemptRestore(ctx)
    }

    return nil
}

func (h *DatabaseFailureHandler) attemptRestore(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if h.testDatabase() == nil {
                h.restoreFromBackup()
                h.readonly.Disable()
                return
            }
        }
    }
}
```

##### 存储故障处理

```go
type StorageFailureHandler struct {
    tempStorage  *TempStorage
    compression  *Compressor
    retention    *RetentionManager
}

func (h *StorageFailureHandler) Handle(ctx context.Context,
    failure Failure) error {
    switch failure.Type {
    case FailureDisk:
        // 磁盘空间不足
        if failure.Details == "disk_full" {
            // 1. 清理过期数据
            h.retention.ForceCleanup()

            // 2. 压缩历史数据
            h.compression.CompressOldData()

            // 3. 切换到临时存储
            h.tempStorage.Enable()
        }

    case FailureStorage:
        // VictoriaMetrics故障
        // 切换到降级模式，仅保存最新数据
        return h.enableDegradedMode()
    }

    return nil
}
```

##### 网络故障处理

```go
type NetworkFailureHandler struct {
    offline   *OfflineMode
    queue     *PersistentQueue
    retry     *RetryManager
}

func (h *NetworkFailureHandler) Handle(ctx context.Context,
    failure Failure) error {
    // 1. 启用离线模式
    h.offline.Enable()

    // 2. 本地队列缓存数据
    h.queue.EnablePersistentMode()

    // 3. 配置重试策略
    h.retry.Configure(RetryConfig{
        MaxAttempts: 10,
        BackoffStrategy: "exponential",
        MaxBackoff: 5 * time.Minute,
    })

    return nil
}
```

### 12.2 数据备份与恢复

#### 备份策略

```go
type BackupManager struct {
    scheduler  *BackupScheduler
    storage    []BackupStorage
    encryption *EncryptionManager
    compression *CompressionManager
}

// 备份配置
type BackupConfig struct {
    // 备份策略
    Schedule struct {
        Full        string `yaml:"full"`         // "0 2 * * 0" 每周日2点
        Incremental string `yaml:"incremental"`  // "0 2 * * 1-6" 每天2点
        Config      string `yaml:"config"`      // "0 */4 * * *" 每4小时
    }

    // 保留策略
    Retention struct {
        Full        int `yaml:"full"`         // 保留4个全量备份
        Incremental int `yaml:"incremental"`  // 保留14个增量备份
        Config      int `yaml:"config"`      // 保留100个配置备份
    }

    // 存储配置
    Storage struct {
        Local   LocalStorage `yaml:"local"`
        S3      S3Storage    `yaml:"s3"`
        SFTP    SFTPStorage  `yaml:"sftp"`
    }

    // 安全配置
    Security struct {
        Encryption bool   `yaml:"encryption"`
        Password   string `yaml:"password"`
        KeyFile    string `yaml:"key_file"`
    }
}
```

#### 备份实现

```go
// 全量备份
func (bm *BackupManager) CreateFullBackup(ctx context.Context) error {
    backup := &Backup{
        ID:        generateBackupID(),
        Type:      BackupTypeFull,
        Timestamp: time.Now(),
        Status:    BackupStatusInProgress,
    }

    // 1. 备份数据库
    dbBackup, err := bm.backupDatabase(ctx)
    if err != nil {
        return fmt.Errorf("database backup failed: %w", err)
    }
    backup.Files = append(backup.Files, dbBackup)

    // 2. 备份时序数据
    tsBackup, err := bm.backupTimeSeries(ctx)
    if err != nil {
        return fmt.Errorf("time series backup failed: %w", err)
    }
    backup.Files = append(backup.Files, tsBackup)

    // 3. 备份配置文件
    configBackup, err := bm.backupConfig(ctx)
    if err != nil {
        return fmt.Errorf("config backup failed: %w", err)
    }
    backup.Files = append(backup.Files, configBackup)

    // 4. 压缩和加密
    if err := bm.compressAndEncrypt(backup); err != nil {
        return fmt.Errorf("compression/encryption failed: %w", err)
    }

    // 5. 上传到存储
    if err := bm.uploadBackup(ctx, backup); err != nil {
        return fmt.Errorf("upload failed: %w", err)
    }

    backup.Status = BackupStatusCompleted
    return bm.saveBackupMetadata(backup)
}

// 增量备份
func (bm *BackupManager) CreateIncrementalBackup(ctx context.Context) error {
    lastBackup := bm.getLastBackup()
    if lastBackup == nil {
        return bm.CreateFullBackup(ctx)
    }

    backup := &Backup{
        ID:         generateBackupID(),
        Type:       BackupTypeIncremental,
        BaseBackup: lastBackup.ID,
        Timestamp:  time.Now(),
        Status:     BackupStatusInProgress,
    }

    // 仅备份变更数据
    changes, err := bm.detectChanges(lastBackup.Timestamp)
    if err != nil {
        return err
    }

    if len(changes) == 0 {
        backup.Status = BackupStatusSkipped
        return nil
    }

    // 备份变更文件
    for _, change := range changes {
        file, err := bm.backupFile(change)
        if err != nil {
            return err
        }
        backup.Files = append(backup.Files, file)
    }

    return bm.finalizeBackup(ctx, backup)
}
```

#### 恢复机制

```go
// 灾难恢复
func (bm *BackupManager) RestoreFromBackup(ctx context.Context,
    backupID string) error {
    backup, err := bm.getBackup(backupID)
    if err != nil {
        return err
    }

    // 1. 停止所有服务
    if err := bm.stopServices(); err != nil {
        return err
    }

    // 2. 下载备份文件
    if err := bm.downloadBackup(ctx, backup); err != nil {
        return err
    }

    // 3. 解密和解压缩
    if err := bm.decryptAndDecompress(backup); err != nil {
        return err
    }

    // 4. 恢复数据库
    if err := bm.restoreDatabase(backup); err != nil {
        return err
    }

    // 5. 恢复时序数据
    if err := bm.restoreTimeSeries(backup); err != nil {
        return err
    }

    // 6. 恢复配置文件
    if err := bm.restoreConfig(backup); err != nil {
        return err
    }

    // 7. 重启服务
    return bm.startServices()
}

// 验证备份完整性
func (bm *BackupManager) VerifyBackup(ctx context.Context,
    backupID string) error {
    backup, err := bm.getBackup(backupID)
    if err != nil {
        return err
    }

    // 1. 验证文件完整性
    for _, file := range backup.Files {
        if err := bm.verifyFileIntegrity(file); err != nil {
            return fmt.Errorf("file integrity check failed: %w", err)
        }
    }

    // 2. 验证数据一致性
    if err := bm.verifyDataConsistency(backup); err != nil {
        return fmt.Errorf("data consistency check failed: %w", err)
    }

    return nil
}
```

### 12.3 高可用部署方案

#### 主备模式

```yaml
# 主备配置
ha_config:
  mode: "active_passive"

  # 主节点
  primary:
    address: "watchdog-primary:8080"
    priority: 100

  # 备节点
  secondary:
    address: "watchdog-secondary:8080"
    priority: 50

  # 健康检查
  health_check:
    interval: "10s"
    timeout: "5s"
    retries: 3

  # 故障切换
  failover:
    automatic: true
    timeout: "30s"

  # 数据同步
  sync:
    interval: "1m"
    method: "rsync"
```

#### 负载均衡模式

```yaml
# 负载均衡配置
load_balancer:
  algorithm: "round_robin" # round_robin, least_conn, ip_hash

  backends:
    - address: "watchdog-1:8080"
      weight: 1
      max_fails: 3
      fail_timeout: "30s"

    - address: "watchdog-2:8080"
      weight: 1
      max_fails: 3
      fail_timeout: "30s"

  health_check:
    uri: "/health"
    interval: "10s"
    timeout: "5s"
```
