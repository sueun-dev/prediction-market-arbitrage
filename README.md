# Prediction Market Arbitrage Scanner

Real-time cross-platform arbitrage scanner for prediction markets. Detects risk-free profit opportunities by finding price discrepancies between platforms.

## What This Does

Monitors multiple prediction market platforms simultaneously and alerts when you can buy YES on one platform + NO on another for less than $1 total. Since one outcome will always pay $1 at settlement, this locks in guaranteed profit.

**Example opportunity detected:**
```
Predict YES = $0.009 + Polymarket NO = $0.969 = $0.978 total
→ Net profit: 1.21% (after fees)
```

## Supported Platforms

| Platform | Connection | Fee |
|----------|-----------|-----|
| Predict.fun | WebSocket | 2% |
| Polymarket | WebSocket | 1% |
| Opinion.Trade | REST Polling | 0.5% |
| Drift BET | WebSocket | 0.5% |
| Myriad Markets | REST Polling | 1% |
| SX Bet | REST Polling | 1% |

## Getting Started

```bash
# Build
go build -o arb ./cmd/arb

# Run with dashboard
./arb --pairs ./site/data/markets_pairs.json

# Run with JSON output (for logging)
./arb --pairs ./site/data/markets_pairs.json --display json --min-profit 0

# With Opinion API key
./arb --pairs ./site/data/markets_pairs.json --opinion-api-key YOUR_KEY
```

## Options

```
--pairs <path>          Path to markets_pairs.json (required)
--min-profit <bps>      Minimum net profit in basis points (default: 50 = 0.5%)
--display <mode>        dashboard, console, or json (default: dashboard)
--cooldown <ms>         Signal dedup cooldown (default: 5000)
--predict-fee <bps>     Predict taker fee (default: 200)
--poly-fee <bps>        Polymarket taker fee (default: 100)
```

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Predict WS │     │ Polymarket  │     │   Opinion   │
│             │     │     WS      │     │   Poller    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                    ┌──────▼──────┐
                    │   Engine    │  ← Arbitrage detection
                    │ (goroutine) │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  Dashboard  │  ← Real-time display
                    └─────────────┘
```

## Tech Stack

- **Go** - High-performance concurrent processing
- **gorilla/websocket** - Real-time price feeds
- **Channel-based architecture** - Lock-free message passing

---

# 예측 마켓 차익거래 스캐너

여러 예측 마켓 플랫폼 간의 가격 차이를 실시간으로 감지하는 무위험 차익거래 스캐너입니다.

## 작동 원리

여러 플랫폼을 동시에 모니터링하여 **A 플랫폼 YES + B 플랫폼 NO < $1** 인 경우를 찾습니다. 만기 시 둘 중 하나는 반드시 $1이 되므로, 진입 비용이 $1 미만이면 무위험 수익이 확정됩니다.

**실제 감지된 기회:**
```
Predict YES = $0.009 + Polymarket NO = $0.969 = 총 $0.978
→ 순수익: 1.21% (수수료 차감 후)
```

## 지원 플랫폼

| 플랫폼 | 연결 방식 | 수수료 |
|--------|----------|--------|
| Predict.fun | WebSocket | 2% |
| Polymarket | WebSocket | 1% |
| Opinion.Trade | REST 폴링 | 0.5% |
| Drift BET | WebSocket | 0.5% |
| Myriad Markets | REST 폴링 | 1% |
| SX Bet | REST 폴링 | 1% |

## 빠른 시작

```bash
# 빌드
go build -o arb ./cmd/arb

# 대시보드 모드로 실행
./arb --pairs ./site/data/markets_pairs.json

# JSON 출력 (로깅용)
./arb --pairs ./site/data/markets_pairs.json --display json --min-profit 0
```

## 주요 옵션

```
--pairs <경로>          markets_pairs.json 경로 (필수)
--min-profit <bps>      최소 순이익 기준점 (기본값: 50 = 0.5%)
--display <모드>        dashboard, console, json (기본값: dashboard)
--cooldown <ms>         중복 신호 제거 간격 (기본값: 5000)
```

## 차익거래 전략

```
크로스 플랫폼 보완 전략 (Cross-Platform Complement)

조건: Platform_A의 YES Ask + Platform_B의 NO Ask < $1

예시:
  Predict YES Ask  = $0.40
  Polymarket NO Ask = $0.55
  총 비용           = $0.95

만기 시:
  - YES 승리 → YES=$1, NO=$0 → 수익 $0.05
  - NO 승리  → YES=$0, NO=$1 → 수익 $0.05

→ 결과에 관계없이 $0.05 확정 수익 (5.26% 수익률)
```

## 기술 스택

- **Go** - 고성능 동시성 처리
- **gorilla/websocket** - 실시간 가격 피드
- **채널 기반 아키텍처** - 락 없는 메시지 패싱
