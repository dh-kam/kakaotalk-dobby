# KakaoBot (카카오톡 메시지 전송 봇)

Golang으로 구현된 카카오톡 REST API 메시지 발송 CLI 및 라이브러리입니다.
서버 모니터링 알림, CI/CD 배포 완료 알림, Cron 작업 결과 전송, 웹훅 릴레이 등을 카카오톡으로 간편하게 전송할 수 있습니다.

---

## ✨ 주요 기능

- 🔐 **OAuth 2.0 자동 로그인 & 토큰 관리**:
  - 로컬 브라우저 자동 오픈 및 콜백 서버를 통한 손쉬운 최초 로그인 (`kakaobot auth login`).
  - Access Token 만료 시 Refresh Token을 이용한 **자동 갱신(Auto Refresh)**.
- 💬 **나에게 보내기 (기본 메시지 / 피드 / 커스텀 템플릿)**:
  - 텍스트, 링크, 버튼, 이미지 피드 메시지 전송.
  - Stdin 파이프라인 지원 (`echo "알림" | kakaobot send me` 또는 `make build | kakaobot send me`).
- 👥 **친구에게 보내기 & 친구 목록 조회**:
  - 앱에 동의한 카카오톡 친구 목록 조회 (`kakaobot friends list`) 및 메시지 발송.
- 🌐 **웹훅 릴레이 서버 (`kakaobot serve`)**:
  - Prometheus Alertmanager, Grafana, GitHub Actions, curl 등 외부 시스템에서 HTTP POST로 카카오톡 메시지를 릴레이.

---

## 🛠 사전 준비: 카카오 개발자 콘솔 설정

카카오톡 메시지를 보내려면 먼저 [카카오 디벨로퍼스(Kakao Developers)](https://developers.kakao.com/)에서 앱을 생성하고 권한을 설정해야 합니다.

1. **애플리케이션 추가**:
   - [Kakao Developers 콘솔](https://developers.kakao.com/) 로그인 후 `내 애플리케이션` > `애플리케이션 추가하기`
   - 앱 이름, 사업자명 입력 후 생성

2. **REST API 키 확인**:
   - `앱 설정` > `요약 정보` 에서 **REST API 키** 확인 (예: `4a8xxxxxxxxxxxxxxxxxxxxxxxxxxxxx`)

3. **플랫폼 등록**:
   - `앱 설정` > `플랫폼` > `Web 플랫폼 등록`
   - 사이트 도메인에 `http://localhost:8080` 등록

4. **카카오 로그인 활성화 & Redirect URI 등록**:
   - `제품 설정` > `카카오 로그인` > `활성화 설정`을 **ON**으로 변경
   - `Redirect URI 등록` 클릭 후 `http://localhost:8080/callback` 추가

5. **동의 항목 설정**:
   - `제품 설정` > `카카오 로그인` > `동의항목` 메뉴 이동
   - **카카오톡 메시지 전송 (`talk_message`)**: `접근권한 관리` > **선택 동의** (또는 필수) 설정
   - **프로필 정보 (`profile_nickname`)**: **필수 동의** 또는 **선택 동의**
   - *(친구에게 보낼 경우)* **카카오 서비스 내 친구 목록 (`friends`)**: **선택 동의** 설정

6. *(선택사항) 보안 설정*:
   - `제품 설정` > `카카오 로그인` > `보안`에서 Client Secret 생성 시 `KAKAO_CLIENT_SECRET`으로 전달 가능.

---

## 🚀 빌드 및 실행

### 1. 빌드 (Makefile)
```bash
# Debug 바이너리 빌드 (build/linux-amd64/debug/kakaobot)
make linux-amd64-debug

# Static Release 바이너리 빌드 (build/linux-amd64/release/kakaobot)
make linux-amd64-release

# 또는 현재 플랫폼 대상 빌드
go build -o kakaobot .
```

### 2. 환경변수 설정 (권장)
매번 CLI 옵션으로 `--client-id`를 넘기지 않도록 `~/.bashrc` 또는 `~/.zshrc`에 등록합니다:

```bash
export KAKAO_REST_API_KEY="your_rest_api_key_here"
# 선택 사항:
# export KAKAO_CLIENT_SECRET="your_client_secret_here"
# export KAKAO_REDIRECT_URI="http://localhost:8080/callback"
# export KAKAO_TOKEN_PATH="~/.config/kakao-bot/tokens.json"
```

---

## 📖 사용 가이드

### 1. 최초 인증 (OAuth 2.0 로그인)
```bash
# 로컬 브라우저가 열리며 카카오 로그인 및 메시지 전송 권한 동의를 진행합니다.
kakaobot auth login

# 수동으로 코드를 입력하여 인증하고 싶은 경우 (원격 서버 / Headless 환경)
kakaobot auth login --manual
```
> 로그인 성공 시 토큰 정보가 `~/.config/kakao-bot/tokens.json`에 안전하게 저장됩니다.

### 2. 인증 상태 확인 및 프로필 조회
```bash
kakaobot auth status
```
```text
KakaoTalk Authentication Status:
  User ID:         123456789
  Nickname:        홍길동
  Token File:      /home/user/.config/kakao-bot/tokens.json
  Token Scope:     talk_message friends profile_nickname
  Access Expired:  false
  Refresh Expired: false
```

### 3. 나에게 메시지 보내기

#### 기본 텍스트 메시지
```bash
kakaobot send me "🚀 서버 배포가 정상적으로 완료되었습니다!"
```

#### Stdin 파이프로 메시지 전송 (모니터링 / 스크립트 연동)
```bash
# 빌드 로그 또는 명령어 결과를 카카오톡으로 전송
make test 2>&1 | kakaobot send me

# 디스크 사용량 알림
df -h | kakaobot send me
```

#### 링크 및 버튼 포함 메시지
```bash
kakaobot send me "새로운 공지사항이 등록되었습니다." \
  --url "https://example.com/notice/1" \
  --button "공지 확인하기"
```

#### 이미지 피드(Feed) 카드 메시지
```bash
kakaobot send me \
  --title "일일 시스템 모니터링 보고서" \
  --description "CPU: 12%, Memory: 45%, Storage: 60%" \
  --image-url "https://via.placeholder.com/600x400.png" \
  --url "https://grafana.example.com" \
  --button "대시보드 바로가기"
```

---

### 4. 친구에게 메시지 보내기

#### 친구 목록 조회 (UUID 확인)
```bash
kakaobot friends list
```
```text
Found 2 KakaoTalk Friend(s) (Total: 2):
UUID                                 | Nickname             | Messageable
-------------------------------------+----------------------+------------
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx | 김철수               | Yes
yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy | 이영희               | Yes
```

#### 친구에게 메시지 발송
```bash
kakaobot send friend "회의실 3층으로 모여주세요." -R "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

---

### 5. 웹훅 릴레이 서버 실행 (`kakaobot serve`)

외부 알림(Alertmanager, GitHub Webhook, Jenkins, curl)을 받아 카카오톡으로 전달하는 백그라운드 HTTP 서버를 구동할 수 있습니다.

```bash
# 기본 포트(127.0.0.1:8080)로 웹훅 서버 구동
kakaobot serve --listen ":8080"
```

#### 웹훅 발송 예시 (curl)
```bash
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "text": "🚨 [긴급] Production 서버 CPU 사용률 90% 초과!",
    "url": "https://grafana.example.com/d/alerts",
    "button_title": "Grafana 보기"
  }'
```

---

## 💻 Go 코드에서 라이브러리로 사용하기

`pkg/kakao` 패키지를 가져와 자신의 Go 프로젝트에서 직접 카카오 API 클라이언트로 활용할 수 있습니다.

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dh-kam/kakao-bot/pkg/kakao"
)

func main() {
	ctx := context.Background()

	// 1. OAuth 클라이언트 & 토큰 저장소 초기화
	oauthClient := kakao.NewOAuthClient(kakao.OAuthConfig{
		ClientID: "YOUR_KAKAO_REST_API_KEY",
	})
	tokenStore := kakao.NewFileTokenStore(kakao.DefaultTokenPath())

	// 2. 카카오 API 클라이언트 생성 (토큰 만료 시 자동 갱신 지원)
	client := kakao.NewClient(kakao.ClientConfig{
		OAuthClient: oauthClient,
		TokenStore:  tokenStore,
	})

	// 3. 나에게 메시지 전송
	err := client.SendMeText(ctx, "Go 프로그램에서 전송한 카카오톡 알림입니다.", "https://github.com", "자세히 보기")
	if err != nil {
		log.Fatalf("메시지 전송 실패: %v", err)
	}

	fmt.Println("카카오톡 메시지 전송 성공!")
}
```

---

## 🧪 테스트 실행

```bash
go test -v ./...
```
