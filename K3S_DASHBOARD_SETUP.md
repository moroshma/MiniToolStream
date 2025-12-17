# Kubernetes Dashboard Setup для k3s кластера

Эта инструкция описывает установку и настройку Kubernetes Dashboard для k3s кластера с использованием k3d.

## Предварительные требования

- Docker Desktop установлен и запущен
- k3d установлен (`brew install k3d`)
- kubectl установлен (`brew install kubectl`)

## Шаг 1: Проверка Docker

```bash
docker info
```

Убедитесь, что Docker запущен и работает корректно.

## Шаг 2: Создание k3s кластера (если не существует)

```bash
# Создание кластера с 1 server и 2 agent нодами
k3d cluster create minitoolstream \
  --servers 1 \
  --agents 2 \
  --port "6550:6443@server:0" \
  --api-port 6550
```

Параметры:
- `--servers 1` - одна control-plane нода
- `--agents 2` - две worker ноды
- `--port "6550:6443@server:0"` - проброс API server порта
- `--api-port 6550` - доступ к Kubernetes API на порту 6550

## Шаг 3: Проверка кластера

```bash
# Список кластеров
k3d cluster list

# Проверка подключения
kubectl cluster-info

# Список нод
kubectl get nodes
```

Ожидаемый вывод:
```
NAME                          STATUS   ROLES                  AGE   VERSION
k3d-minitoolstream-server-0   Ready    control-plane,master   45h   v1.33.4+k3s1
k3d-minitoolstream-agent-0    Ready    <none>                 45h   v1.33.4+k3s1
k3d-minitoolstream-agent-1    Ready    <none>                 45h   v1.33.4+k3s1
```

## Шаг 4: Установка Kubernetes Dashboard

```bash
# Установка Dashboard v2.7.0
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

# Ожидание готовности
kubectl wait --for=condition=available --timeout=60s deployment/kubernetes-dashboard -n kubernetes-dashboard
```

Эта команда создаст:
- Namespace: `kubernetes-dashboard`
- Deployment с Dashboard UI
- Service для доступа к Dashboard
- Необходимые RBAC роли и ConfigMaps

## Шаг 5: Создание ServiceAccount с правами администратора

Файл `k8s-dashboard-admin.yaml`:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: admin-user
  namespace: kubernetes-dashboard
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: admin-user
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: admin-user
  namespace: kubernetes-dashboard
```

Применение конфигурации:

```bash
kubectl apply -f k8s-dashboard-admin.yaml
```

## Шаг 6: Получение токена доступа

```bash
# Генерация токена (действителен 10 лет)
kubectl create token admin-user -n kubernetes-dashboard --duration=87600h > k8s-dashboard-token.txt

# Просмотр токена
cat k8s-dashboard-token.txt
```

## Шаг 7: Запуск Dashboard

### Вариант А: Использование готового скрипта

```bash
./start-dashboard.sh
```

Скрипт автоматически:
1. Покажет токен для входа
2. Запустит port-forward
3. Сообщит адрес доступа

### Вариант Б: Ручной запуск

```bash
kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard 8443:443
```

## Шаг 8: Доступ к Dashboard

1. Откройте браузер и перейдите по адресу: **https://localhost:8443**

2. Браузер покажет предупреждение о безопасности (самоподписанный сертификат):
   - Chrome: нажмите "Advanced" → "Proceed to localhost"
   - Firefox: нажмите "Advanced" → "Accept the Risk and Continue"
   - Safari: нажмите "Show Details" → "visit this website"

3. Выберите метод входа: **Token**

4. Вставьте токен из файла `k8s-dashboard-token.txt` и нажмите "Sign In"

## Архитектура решения

```
┌─────────────────┐
│   Browser       │
│  localhost:8443 │
└────────┬────────┘
         │ HTTPS
         │
┌────────▼────────────────────────────────┐
│   kubectl port-forward                  │
│   (проксирует трафик в кластер)         │
└────────┬────────────────────────────────┘
         │
┌────────▼────────────────────────────────┐
│   kubernetes-dashboard Service          │
│   (namespace: kubernetes-dashboard)     │
└────────┬────────────────────────────────┘
         │
┌────────▼────────────────────────────────┐
│   kubernetes-dashboard Pod              │
│   (Dashboard UI приложение)             │
└─────────────────────────────────────────┘
```

## Полезные команды

### Управление кластером

```bash
# Список кластеров
k3d cluster list

# Остановка кластера
k3d cluster stop minitoolstream

# Запуск кластера
k3d cluster start minitoolstream

# Удаление кластера
k3d cluster delete minitoolstream
```

### Проверка Dashboard

```bash
# Статус компонентов Dashboard
kubectl get all -n kubernetes-dashboard

# Логи Dashboard
kubectl logs -n kubernetes-dashboard deployment/kubernetes-dashboard

# Проверка ServiceAccount
kubectl get serviceaccount admin-user -n kubernetes-dashboard

# Проверка ClusterRoleBinding
kubectl get clusterrolebinding admin-user
```

### Обновление токена

```bash
# Создание нового токена (например, на 30 дней)
kubectl create token admin-user -n kubernetes-dashboard --duration=720h > k8s-dashboard-token.txt
```

### Остановка port-forward

```bash
# Найти процесс
ps aux | grep "port-forward.*kubernetes-dashboard"

# Остановить (или Ctrl+C в терминале где запущен)
kill <PID>
```

## Troubleshooting

### Dashboard не отвечает

```bash
# Проверка статуса Pod
kubectl get pods -n kubernetes-dashboard

# Если Pod не в статусе Running
kubectl describe pod -n kubernetes-dashboard <pod-name>

# Перезапуск Dashboard
kubectl rollout restart deployment/kubernetes-dashboard -n kubernetes-dashboard
```

### Ошибка токена "Invalid token"

```bash
# Пересоздать токен
kubectl create token admin-user -n kubernetes-dashboard --duration=87600h > k8s-dashboard-token.txt

# Проверить ServiceAccount
kubectl get serviceaccount admin-user -n kubernetes-dashboard -o yaml
```

### Порт 8443 уже занят

```bash
# Использовать другой локальный порт
kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard 9443:443

# Теперь доступ через https://localhost:9443
```

### Кластер не запускается

```bash
# Проверить Docker
docker ps

# Логи k3d
k3d cluster list
docker logs k3d-minitoolstream-server-0

# Пересоздать кластер
k3d cluster delete minitoolstream
k3d cluster create minitoolstream --servers 1 --agents 2 --port "6550:6443@server:0"
```

## Безопасность

### Важные замечания

1. **Токен имеет полные права администратора** - не передавайте его посторонним
2. **Токен действителен 10 лет** - храните файл `k8s-dashboard-token.txt` в безопасном месте
3. **Dashboard доступен только локально** - используется `localhost`, внешнего доступа нет
4. **Самоподписанный сертификат** - нормально для локальной разработки

### Рекомендации для production

Для production окружения:
- Используйте токены с коротким временем жизни
- Настройте Ingress с TLS сертификатом от Let's Encrypt
- Используйте OAuth2/OIDC для аутентификации
- Ограничьте права доступа через RBAC
- Включите аудит логирование

## Что установлено

После выполнения всех шагов в кластере установлено:

1. **Kubernetes Dashboard** (v2.7.0)
   - Namespace: `kubernetes-dashboard`
   - UI для управления кластером

2. **ServiceAccount** `admin-user`
   - Полные права администратора (cluster-admin)
   - Используется для входа в Dashboard

3. **Токен доступа**
   - Сохранен в `k8s-dashboard-token.txt`
   - Срок действия: 10 лет

4. **Port-forward**
   - Локальный доступ через https://localhost:8443
   - Безопасный HTTPS туннель в кластер

## Дополнительные ресурсы

- [Kubernetes Dashboard GitHub](https://github.com/kubernetes/dashboard)
- [k3d документация](https://k3d.io/)
- [k3s документация](https://k3s.io/)
- [kubectl документация](https://kubernetes.io/docs/reference/kubectl/)

## Автор

Создано: 2025-01-05
Версия: 1.0
