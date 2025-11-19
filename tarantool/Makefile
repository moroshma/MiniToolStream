# Makefile для управления Tarantool в Kubernetes

NAMESPACE := minitoolstream
CONFIGMAP := tarantool-init

.PHONY: help
help: ## Показать справку
	@echo "Доступные команды:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: configmap
configmap: ## Создать ConfigMap из локального файла init.lua
	@echo "Создание ConfigMap из init.lua..."
	@kubectl delete configmap $(CONFIGMAP) -n $(NAMESPACE) 2>/dev/null || true
	@kubectl create configmap $(CONFIGMAP) \
		--from-file=init.lua=init.lua \
		-n $(NAMESPACE)
	@echo "✅ ConfigMap создан"

.PHONY: apply
apply: configmap ## Применить всю конфигурацию (configmap + statefulset + services)
	@echo "Применение конфигурации..."
	@kubectl apply -f k8s/statefulset.yaml
	@kubectl apply -f k8s/service.yaml
	@echo "✅ Конфигурация применена"

.PHONY: restart
restart: ## Перезапустить StatefulSet для применения изменений
	@echo "Перезапуск подов..."
	@kubectl rollout restart statefulset/tarantool -n $(NAMESPACE)
	@echo "✅ Перезапуск инициирован"
	@echo "Проверьте статус: make status"

.PHONY: update
update: configmap restart ## Обновить init.lua и перезапустить поды
	@echo "✅ Обновление завершено"

.PHONY: status
status: ## Показать статус подов и services
	@echo "=== Pods ==="
	@kubectl get pods -n $(NAMESPACE) -o wide
	@echo ""
	@echo "=== Services ==="
	@kubectl get svc -n $(NAMESPACE)

.PHONY: logs
logs: ## Показать логи пода
	@kubectl logs tarantool-0 -n $(NAMESPACE) --tail=50

.PHONY: logs-follow
logs-follow: ## Логи в реальном времени
	@kubectl logs -f tarantool-0 -n $(NAMESPACE)

.PHONY: shell
shell: ## Подключиться к поду
	@kubectl exec -it tarantool-0 -n $(NAMESPACE) -- /bin/sh

.PHONY: console
console: ## Консоль Tarantool
	@kubectl exec -it tarantool-0 -n $(NAMESPACE) -- tarantoolctl connect localhost:3301

.PHONY: port-forward
port-forward: ## Port-forward на tarantool (localhost:3301)
	@echo "Port-forwarding tarantool-service на localhost:3301"
	@echo "Нажмите Ctrl+C для остановки"
	@kubectl port-forward -n $(NAMESPACE) svc/tarantool-service 3301:3301

.PHONY: test
test: ## Запустить Go тесты
	@echo "Запуск тестов..."
	@echo "Убедитесь что запущен port-forward: make port-forward"
	@go run test_new_schema.go

.PHONY: clean
clean: ## Удалить все ресурсы
	@echo "⚠️  Удаление всех ресурсов Tarantool..."
	@read -p "Вы уверены? (y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	@kubectl delete statefulset tarantool -n $(NAMESPACE) || true
	@kubectl delete svc tarantool tarantool-service -n $(NAMESPACE) || true
	@kubectl delete configmap $(CONFIGMAP) -n $(NAMESPACE) || true
	@echo "✅ Ресурсы удалены (PVC сохранены)"

.PHONY: clean-all
clean-all: clean ## Удалить все включая PVC (данные будут потеряны!)
	@echo "⚠️  ВНИМАНИЕ: Это удалит все данные!"
	@read -p "Вы ТОЧНО уверены? Введите 'delete-all-data': " confirm && [ "$$confirm" = "delete-all-data" ] || exit 1
	@kubectl delete pvc -l app=tarantool -n $(NAMESPACE) || true
	@echo "✅ Все данные удалены"

.PHONY: deploy
deploy: ## Полное развертывание с нуля
	@echo "Развертывание Tarantool (standalone)..."
	@kubectl apply -f k8s/namespace.yaml
	@make configmap
	@kubectl apply -f k8s/statefulset.yaml
	@kubectl apply -f k8s/service.yaml
	@echo "✅ Развертывание завершено"
	@echo "Проверьте статус: make status"

.PHONY: watch
watch: ## Следить за подами в реальном времени
	@watch -n 2 'kubectl get pods -n $(NAMESPACE) -o wide'

.PHONY: describe
describe: ## Подробная информация о ресурсах
	@echo "=== StatefulSet ==="
	@kubectl describe statefulset tarantool -n $(NAMESPACE)
	@echo ""
	@echo "=== Pods ==="
	@kubectl describe pods -l app=tarantool -n $(NAMESPACE)

.PHONY: events
events: ## Показать события
	@kubectl get events -n $(NAMESPACE) --sort-by='.lastTimestamp' | tail -20

.PHONY: top
top: ## Использование ресурсов
	@kubectl top pods -n $(NAMESPACE)
