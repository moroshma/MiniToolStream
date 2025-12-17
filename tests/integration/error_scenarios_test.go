package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	pb "github.com/moroshma/MiniToolStreamConnector/model"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// TestErrorScenarios_ConnectionFailure проверяет обработку ошибок подключения
func TestErrorScenarios_ConnectionFailure(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ИНТЕГРАЦИОННЫЙ ТЕСТ 3: CONNECTION FAILURES (Ошибки подключения)")
	fmt.Println(strings.Repeat("=", 80))

	t.Run("Invalid_Server_Address", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 3.1: Подключение к несуществующему серверу")

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Пытаемся подключиться к несуществующему адресу
		conn, err := grpc.DialContext(
			ctx,
			"localhost:99999",
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)

		if err != nil {
			fmt.Printf("  ✓ УСПЕХ: Подключение к несуществующему серверу failed: %v\n", err)
		} else {
			conn.Close()
			t.Errorf("  ✗ ОШИБКА: Подключение к несуществующему серверу не должно было пройти")
		}
	})

	t.Run("Connection_Timeout", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 3.2: Таймаут подключения")

		// Короткий таймаут для неотвечающего сервера
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		conn, err := grpc.DialContext(
			ctx,
			"10.255.255.1:50051", // Non-routable address
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
		)

		if err != nil {
			fmt.Printf("  ✓ УСПЕХ: Подключение завершилось с таймаутом: %v\n", err)
		} else {
			conn.Close()
			t.Errorf("  ✗ ОШИБКА: Подключение не должно было пройти")
		}
	})
}

// TestErrorScenarios_InvalidRequests проверяет обработку невалидных запросов
func TestErrorScenarios_InvalidRequests(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ИНТЕГРАЦИОННЫЙ ТЕСТ 4: INVALID REQUESTS (Невалидные запросы)")
	fmt.Println(strings.Repeat("=", 80))

	ctx := context.Background()

	conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
		return
	}
	defer conn.Close()

	ingressClient := pb.NewIngressServiceClient(conn)

	t.Run("Empty_Subject", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 4.1: Публикация с пустым subject")

		resp, err := ingressClient.Publish(ctx, &pb.PublishRequest{
			Subject: "",
			Data:    []byte("test data"),
		})

		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.InvalidArgument {
				fmt.Printf("  ✓ УСПЕХ: Запрос отклонен с кодом InvalidArgument: %v\n", st.Message())
			} else {
				fmt.Printf("  ✓ УСПЕХ: Запрос отклонен: %v\n", err)
			}
		} else if resp.StatusCode != 0 {
			fmt.Printf("  ✓ УСПЕХ: Запрос обработан с ошибкой: %s (status_code=%d)\n", resp.ErrorMessage, resp.StatusCode)
		} else {
			t.Errorf("  ✗ ОШИБКА: Публикация с пустым subject должна была быть отклонена")
		}
	})

	t.Run("Nil_Request", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 4.2: Nil request (gRPC преобразует в пустой message)")
		fmt.Println("  ℹ В gRPC клиенте Go, nil автоматически преобразуется в пустой protobuf message")

		resp, err := ingressClient.Publish(ctx, nil)

		// gRPC клиент Go автоматически конвертирует nil в пустой message
		// Сервер должен отклонить его из-за пустого subject
		if err == nil && resp != nil && resp.StatusCode != 0 && resp.ErrorMessage != "" {
			fmt.Printf("  ✓ УСПЕХ: Пустой message отклонен (status_code=%d, error=%s)\n", resp.StatusCode, resp.ErrorMessage)
		} else if err != nil {
			// В некоторых версиях gRPC может вернуть ошибку напрямую
			fmt.Printf("  ✓ УСПЕХ: Request отклонен: %v\n", err)
		} else {
			t.Errorf("  ✗ ОШИБКА: Пустой request должен был быть отклонен")
		}
	})

	t.Run("Large_Subject_Name", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 4.3: Очень длинное имя subject")

		// Создаем очень длинный subject
		longSubject := strings.Repeat("a", 1000)

		resp, err := ingressClient.Publish(ctx, &pb.PublishRequest{
			Subject: longSubject,
			Data:    []byte("test data"),
		})

		if err != nil {
			fmt.Printf("  ✓ Запрос с длинным subject отклонен: %v\n", err)
		} else if resp.StatusCode != 0 {
			fmt.Printf("  ✓ Запрос обработан с ошибкой: %s\n", resp.ErrorMessage)
		} else {
			fmt.Printf("  ℹ Запрос с длинным subject принят (sequence=%d)\n", resp.Sequence)
		}
	})
}

// TestErrorScenarios_EgressErrors проверяет ошибки Egress сервиса
func TestErrorScenarios_EgressErrors(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ИНТЕГРАЦИОННЫЙ ТЕСТ 5: EGRESS ERRORS (Ошибки при чтении сообщений)")
	fmt.Println(strings.Repeat("=", 80))

	ctx := context.Background()

	conn, err := grpc.Dial(egressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Egress сервис не доступен: %v (тест пропущен)", err)
		return
	}
	defer conn.Close()

	egressClient := pb.NewEgressServiceClient(conn)

	t.Run("Subscribe_Empty_Subject", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 5.1: Subscribe с пустым subject")

		stream, err := egressClient.Subscribe(ctx, &pb.SubscribeRequest{
			Subject:     "",
			DurableName: "test-consumer",
		})

		if err != nil {
			fmt.Printf("  ✓ УСПЕХ: Subscribe отклонен: %v\n", err)
		} else {
			// Пытаемся получить первое сообщение
			_, err := stream.Recv()
			if err != nil {
				fmt.Printf("  ✓ УСПЕХ: Stream завершился с ошибкой: %v\n", err)
			} else {
				t.Errorf("  ✗ ОШИБКА: Subscribe с пустым subject должен был быть отклонен")
			}
		}
	})

	t.Run("Subscribe_Empty_DurableName", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 5.2: Subscribe с пустым durable_name")

		stream, err := egressClient.Subscribe(ctx, &pb.SubscribeRequest{
			Subject:     "test.subject",
			DurableName: "",
		})

		if err != nil {
			fmt.Printf("  ✓ УСПЕХ: Subscribe отклонен: %v\n", err)
		} else {
			_, err := stream.Recv()
			if err != nil {
				fmt.Printf("  ✓ УСПЕХ: Stream завершился с ошибкой: %v\n", err)
			} else {
				t.Errorf("  ✗ ОШИБКА: Subscribe с пустым durable_name должен был быть отклонен")
			}
		}
	})

	t.Run("Fetch_Empty_Subject", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 5.3: Fetch с пустым subject")

		stream, err := egressClient.Fetch(ctx, &pb.FetchRequest{
			Subject:     "",
			DurableName: "test-consumer",
			BatchSize:   10,
		})

		if err != nil {
			fmt.Printf("  ✓ УСПЕХ: Fetch отклонен: %v\n", err)
		} else {
			_, err := stream.Recv()
			if err != nil {
				fmt.Printf("  ✓ УСПЕХ: Stream завершился с ошибкой: %v\n", err)
			} else {
				t.Errorf("  ✗ ОШИБКА: Fetch с пустым subject должен был быть отклонен")
			}
		}
	})

	t.Run("GetLastSequence_Empty_Subject", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 5.4: GetLastSequence с пустым subject")

		_, err := egressClient.GetLastSequence(ctx, &pb.GetLastSequenceRequest{
			Subject: "",
		})

		if err != nil {
			fmt.Printf("  ✓ УСПЕХ: GetLastSequence отклонен: %v\n", err)
		} else {
			t.Errorf("  ✗ ОШИБКА: GetLastSequence с пустым subject должен был быть отклонен")
		}
	})

	t.Run("GetLastSequence_Nonexistent_Subject", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 5.5: GetLastSequence для несуществующего subject")

		resp, err := egressClient.GetLastSequence(ctx, &pb.GetLastSequenceRequest{
			Subject: "nonexistent.subject.12345",
		})

		if err != nil {
			fmt.Printf("  ✓ Запрос завершился с ошибкой: %v\n", err)
		} else {
			fmt.Printf("  ℹ Запрос успешен, last_sequence=%d (может быть 0 для нового subject)\n", resp.LastSequence)
		}
	})
}

// TestErrorScenarios_DataValidation проверяет валидацию данных
func TestErrorScenarios_DataValidation(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("ИНТЕГРАЦИОННЫЙ ТЕСТ 6: DATA VALIDATION (Валидация данных)")
	fmt.Println(strings.Repeat("=", 80))

	ctx := context.Background()

	conn, err := grpc.Dial(ingressAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Skipf("Ingress сервис не доступен: %v (тест пропущен)", err)
		return
	}
	defer conn.Close()

	ingressClient := pb.NewIngressServiceClient(conn)

	t.Run("Empty_Data", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 6.1: Публикация с пустыми данными")

		resp, err := ingressClient.Publish(ctx, &pb.PublishRequest{
			Subject: "test.empty",
			Data:    []byte{},
		})

		if err != nil {
			fmt.Printf("  ℹ Запрос отклонен: %v\n", err)
		} else if resp.StatusCode != 0 {
			fmt.Printf("  ℹ Запрос обработан с ошибкой: %s\n", resp.ErrorMessage)
		} else {
			fmt.Printf("  ✓ Запрос успешен (sequence=%d). Пустые данные допустимы.\n", resp.Sequence)
		}
	})

	t.Run("Nil_Data", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 6.2: Публикация с nil данными")

		resp, err := ingressClient.Publish(ctx, &pb.PublishRequest{
			Subject: "test.nil",
			Data:    nil,
		})

		if err != nil {
			fmt.Printf("  ℹ Запрос отклонен: %v\n", err)
		} else if resp.StatusCode != 0 {
			fmt.Printf("  ℹ Запрос обработан с ошибкой: %s\n", resp.ErrorMessage)
		} else {
			fmt.Printf("  ✓ Запрос успешен (sequence=%d). Nil данные обработаны как пустые.\n", resp.Sequence)
		}
	})

	t.Run("Large_Message", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 6.3: Публикация большого сообщения (10 MB)")

		// Создаем 10 MB сообщение
		largeData := make([]byte, 10*1024*1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		fmt.Printf("  ✓ Создано сообщение размером %d байт\n", len(largeData))

		resp, err := ingressClient.Publish(ctx, &pb.PublishRequest{
			Subject: "test.large",
			Data:    largeData,
		})

		if err != nil {
			fmt.Printf("  ℹ Большое сообщение отклонено: %v\n", err)
		} else if resp.StatusCode != 0 {
			fmt.Printf("  ℹ Запрос обработан с ошибкой: %s\n", resp.ErrorMessage)
		} else {
			fmt.Printf("  ✓ УСПЕХ: Большое сообщение опубликовано (sequence=%d, object=%s)\n",
				resp.Sequence, resp.ObjectName)
		}
	})

	t.Run("Special_Characters_In_Subject", func(t *testing.T) {
		fmt.Println("\n▶ Подтест 6.4: Subject с специальными символами")

		subjects := []string{
			"test.subject-with-dash",
			"test.subject_with_underscore",
			"test.subject.with.dots",
			"test/subject/with/slashes",
		}

		for _, subj := range subjects {
			resp, err := ingressClient.Publish(ctx, &pb.PublishRequest{
				Subject: subj,
				Data:    []byte("test"),
			})

			if err != nil {
				fmt.Printf("  ℹ Subject '%s' отклонен: %v\n", subj, err)
			} else if resp.StatusCode != 0 {
				fmt.Printf("  ℹ Subject '%s' обработан с ошибкой: %s\n", subj, resp.ErrorMessage)
			} else {
				fmt.Printf("  ✓ Subject '%s' принят (sequence=%d)\n", subj, resp.Sequence)
			}
		}
	})
}
