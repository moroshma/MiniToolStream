package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/moroshma/MiniToolStreamConnector/auth"
	pb "github.com/moroshma/MiniToolStreamConnector/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	ingressAddr = "localhost:50051"
	egressAddr  = "localhost:50052"
	vaultAddr   = "http://localhost:8200"
	vaultToken  = "dev-root-token"
	vaultPath   = "secret/data/minitoolstream/jwt"
	issuer      = "minitoolstream"
)

// TestAuthJWTValidation проверяет валидацию JWT токенов
func TestAuthJWTValidation(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ИНТЕГРАЦИОННЫЙ ТЕСТ 1: JWT VALIDATION (Проверка валидации токенов)")
	fmt.Println(strings.Repeat("=", 80))

	ctx := context.Background()

	// Создаем Vault клиент
	config := vault.DefaultConfig()
	config.Address = vaultAddr
	vaultClient, err := vault.NewClient(config)
	if err != nil {
		t.Skipf("Vault не доступен: %v (тест пропущен)", err)
		return
	}
	vaultClient.SetToken(vaultToken)

	// Создаем JWT manager
	jwtManager, err := auth.NewJWTManagerFromVault(ctx, vaultClient, vaultPath, issuer)
	if err != nil {
		t.Skipf("JWT manager не доступен: %v (тест пропущен)", err)
		return
	}

	t.Run("Valid_Token_Success", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 1.1: Валидный токен должен быть принят")

		// Генерируем валидный токен
		token, err := jwtManager.GenerateToken(
			"test-client-valid",
			[]string{"test.*"},
			[]string{"publish", "subscribe", "fetch"},
			1*time.Hour,
		)
		if err != nil {
			t.Fatalf("Не удалось сгенерировать токен: %v", err)
		}

		fmt.Printf("  ✓ Токен сгенерирован для client_id: test-client-valid\n")
		fmt.Printf("  ✓ Разрешенные subjects: test.*\n")
		fmt.Printf("  ✓ Permissions: publish, subscribe, fetch\n")

		// Подключаемся к Ingress
		conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
			return
		}
		defer conn.Close()

		ingressClient := pb.NewIngressServiceClient(conn)

		// Создаем контекст с токеном
		md := metadata.Pairs("authorization", "Bearer "+token)
		authCtx := metadata.NewOutgoingContext(ctx, md)

		// Пытаемся опубликовать сообщение
		resp, err := ingressClient.Publish(authCtx, &pb.PublishRequest{
			Subject: "test.valid",
			Data:    []byte("test data"),
		})

		if err != nil {
			t.Errorf("  ✗ ОШИБКА: Запрос с валидным токеном был отклонен: %v", err)
		} else if resp.StatusCode != 0 {
			t.Errorf("  ✗ ОШИБКА: Неожиданный код ответа: %d, сообщение: %s", resp.StatusCode, resp.ErrorMessage)
		} else {
			fmt.Printf("  ✓ УСПЕХ: Сообщение опубликовано (sequence=%d)\n", resp.Sequence)
		}
	})

	t.Run("Invalid_Signature", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 1.2: Токен с неверной подписью должен быть отклонен")

		// Создаем токен с неверной подписью
		fakeToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbGllbnRfaWQiOiJmYWtlLWNsaWVudCIsImFsbG93ZWRfc3ViamVjdHMiOlsiKiJdLCJwZXJtaXNzaW9ucyI6WyIqIl0sImlzcyI6ImZha2UiLCJleHAiOjk5OTk5OTk5OTl9.fake_signature"

		fmt.Printf("  ✓ Использован токен с неверной подписью\n")

		conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
			return
		}
		defer conn.Close()

		ingressClient := pb.NewIngressServiceClient(conn)

		md := metadata.Pairs("authorization", "Bearer "+fakeToken)
		authCtx := metadata.NewOutgoingContext(ctx, md)

		_, err = ingressClient.Publish(authCtx, &pb.PublishRequest{
			Subject: "test.invalid",
			Data:    []byte("test data"),
		})

		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Unauthenticated {
				fmt.Printf("  ✓ УСПЕХ: Запрос отклонен с кодом Unauthenticated\n")
			} else {
				fmt.Printf("  ✓ УСПЕХ: Запрос отклонен: %v\n", err)
			}
		} else {
			t.Errorf("  ✗ ОШИБКА: Токен с неверной подписью был принят!")
		}
	})

	t.Run("Expired_Token", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 1.3: Истекший токен должен быть отклонен")

		// Генерируем токен с отрицательным временем жизни
		token, err := jwtManager.GenerateToken(
			"test-client-expired",
			[]string{"*"},
			[]string{"*"},
			-1*time.Hour, // Токен уже истек
		)
		if err != nil {
			t.Fatalf("Не удалось сгенерировать токен: %v", err)
		}

		fmt.Printf("  ✓ Токен сгенерирован с истекшим сроком действия\n")

		conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
			return
		}
		defer conn.Close()

		ingressClient := pb.NewIngressServiceClient(conn)

		md := metadata.Pairs("authorization", "Bearer "+token)
		authCtx := metadata.NewOutgoingContext(ctx, md)

		_, err = ingressClient.Publish(authCtx, &pb.PublishRequest{
			Subject: "test.expired",
			Data:    []byte("test data"),
		})

		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Unauthenticated {
				fmt.Printf("  ✓ УСПЕХ: Истекший токен отклонен с кодом Unauthenticated\n")
			} else {
				fmt.Printf("  ✓ УСПЕХ: Истекший токен отклонен: %v\n", err)
			}
		} else {
			t.Errorf("  ✗ ОШИБКА: Истекший токен был принят!")
		}
	})

	t.Run("Missing_Token", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 1.4: Запрос без токена (если авторизация включена)")

		conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
			return
		}
		defer conn.Close()

		ingressClient := pb.NewIngressServiceClient(conn)

		// Запрос без токена
		resp, err := ingressClient.Publish(ctx, &pb.PublishRequest{
			Subject: "test.noauth",
			Data:    []byte("test data"),
		})

		// В зависимости от конфигурации, запрос может быть принят или отклонен
		if err != nil {
			fmt.Printf("  ℹ Запрос без токена отклонен: %v\n", err)
			fmt.Printf("  ℹ (Авторизация обязательна на сервере)\n")
		} else if resp.StatusCode != 0 {
			fmt.Printf("  ℹ Запрос без токена обработан с ошибкой: %s\n", resp.ErrorMessage)
		} else {
			fmt.Printf("  ℹ Запрос без токена принят (sequence=%d)\n", resp.Sequence)
			fmt.Printf("  ℹ (Авторизация опциональна на сервере)\n")
		}
	})
}

// TestAuthPermissions проверяет контроль прав доступа
func TestAuthPermissions(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ИНТЕГРАЦИОННЫЙ ТЕСТ 2: PERMISSIONS (Проверка прав доступа)")
	fmt.Println(strings.Repeat("=", 80))

	ctx := context.Background()

	config := vault.DefaultConfig()
	config.Address = vaultAddr
	vaultClient, err := vault.NewClient(config)
	if err != nil {
		t.Skipf("Vault не доступен: %v (тест пропущен)", err)
		return
	}
	vaultClient.SetToken(vaultToken)

	jwtManager, err := auth.NewJWTManagerFromVault(ctx, vaultClient, vaultPath, issuer)
	if err != nil {
		t.Skipf("JWT manager не доступен: %v (тест пропущен)", err)
		return
	}

	t.Run("Publish_Permission_Denied", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 2.1: Токен без права 'publish' должен быть отклонен")

		// Генерируем токен только с правами subscribe и fetch
		token, err := jwtManager.GenerateToken(
			"test-client-no-publish",
			[]string{"*"},
			[]string{"subscribe", "fetch"}, // Нет "publish"
			1*time.Hour,
		)
		if err != nil {
			t.Fatalf("Не удалось сгенерировать токен: %v", err)
		}

		fmt.Printf("  ✓ Токен сгенерирован с permissions: subscribe, fetch (без publish)\n")

		conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
			return
		}
		defer conn.Close()

		ingressClient := pb.NewIngressServiceClient(conn)

		md := metadata.Pairs("authorization", "Bearer "+token)
		authCtx := metadata.NewOutgoingContext(ctx, md)

		_, err = ingressClient.Publish(authCtx, &pb.PublishRequest{
			Subject: "test.nopublish",
			Data:    []byte("test data"),
		})

		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.PermissionDenied {
				fmt.Printf("  ✓ УСПЕХ: Запрос отклонен с кодом PermissionDenied\n")
			} else {
				fmt.Printf("  ✓ УСПЕХ: Запрос отклонен: %v\n", err)
			}
		} else {
			t.Errorf("  ✗ ОШИБКА: Запрос без права 'publish' был принят!")
		}
	})

	t.Run("Subject_Access_Denied", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 2.2: Доступ к запрещенному subject должен быть отклонен")

		// Генерируем токен с доступом только к "allowed.*"
		token, err := jwtManager.GenerateToken(
			"test-client-restricted",
			[]string{"allowed.*"}, // Только "allowed.*"
			[]string{"publish", "subscribe", "fetch"},
			1*time.Hour,
		)
		if err != nil {
			t.Fatalf("Не удалось сгенерировать токен: %v", err)
		}

		fmt.Printf("  ✓ Токен сгенерирован с доступом только к subjects: allowed.*\n")

		conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
			return
		}
		defer conn.Close()

		ingressClient := pb.NewIngressServiceClient(conn)

		md := metadata.Pairs("authorization", "Bearer "+token)
		authCtx := metadata.NewOutgoingContext(ctx, md)

		// Пытаемся опубликовать в запрещенный subject
		_, err = ingressClient.Publish(authCtx, &pb.PublishRequest{
			Subject: "forbidden.topic",
			Data:    []byte("test data"),
		})

		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.PermissionDenied {
				fmt.Printf("  ✓ УСПЕХ: Запрос к запрещенному subject отклонен с кодом PermissionDenied\n")
			} else {
				fmt.Printf("  ✓ УСПЕХ: Запрос к запрещенному subject отклонен: %v\n", err)
			}
		} else {
			t.Errorf("  ✗ ОШИБКА: Доступ к запрещенному subject был разрешен!")
		}

		// Проверяем разрешенный subject
		resp, err := ingressClient.Publish(authCtx, &pb.PublishRequest{
			Subject: "allowed.topic",
			Data:    []byte("test data"),
		})

		if err != nil {
			t.Errorf("  ✗ ОШИБКА: Запрос к разрешенному subject был отклонен: %v", err)
		} else if resp.StatusCode != 0 {
			t.Errorf("  ✗ ОШИБКА: Неожиданный код ответа: %d", resp.StatusCode)
		} else {
			fmt.Printf("  ✓ УСПЕХ: Доступ к разрешенному subject 'allowed.topic' предоставлен (sequence=%d)\n", resp.Sequence)
		}
	})
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
