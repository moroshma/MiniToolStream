# 5. Развертывание в Kubernetes

## 5.1 Подготовка инфраструктуры Kubernetes

### 5.1.1 Выбор Kubernetes дистрибутива

Для развертывания MiniToolStream выбран **k3s** — облегченный дистрибутив Kubernetes от Rancher Labs, оптимизированный для edge-вычислений и локальной разработки.

**Преимущества k3s:**

1. **Малый размер** — бинарный файл ~70 MB (vs ~1.5 GB у обычного Kubernetes)
2. **Низкое потребление ресурсов** — минимальные требования: 512 MB RAM, 1 CPU core
3. **Простота установки** — одна команда для развертывания
4. **Полная совместимость** — сертифицированный CNCF Kubernetes дистрибутив
5. **Встроенные компоненты** — включает Traefik Ingress, CoreDNS, metrics-server
6. **SQLite вместо etcd** — упрощенное хранилище состояния кластера

**Альтернативы:**
- **Minikube** — для локальной разработки, но тяжелее k3s
- **Kind** — Kubernetes in Docker, для CI/CD
- **MicroK8s** — от Canonical, похож на k3s
- **Managed Kubernetes** — GKE, EKS, AKS для production

### 5.1.2 Использование k3d для локальной разработки

**k3d** — это обертка для запуска k3s в Docker-контейнерах. Позволяет создавать многонодовые кластеры на локальной машине.

**Установка k3d:**

```bash
# macOS
brew install k3d

# Linux
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Проверка версии
k3d version
```

### 5.1.3 Создание кластера

**Архитектура кластера:**

```
┌─────────────────────────────────────────────────────┐
│              k3d-minitoolstream Cluster             │
│                                                     │
│  ┌──────────────┐    ┌──────────────┐              │
│  │   Server 0   │    │   Agent 0    │              │
│  │ (Control)    │    │   (Worker)   │              │
│  │              │    │              │              │
│  │ • API Server │    │ • Kubelet    │              │
│  │ • Scheduler  │    │ • Workloads  │              │
│  │ • Controller │    │              │              │
│  └──────────────┘    └──────────────┘              │
│                      ┌──────────────┐              │
│                      │   Agent 1    │              │
│                      │   (Worker)   │              │
│                      │              │              │
│                      │ • Kubelet    │              │
│                      │ • Workloads  │              │
│                      └──────────────┘              │
│                                                     │
│  LoadBalancer:                                      │
│  - Port 8080 -> 80 (HTTP)                           │
│  - Port 8443 -> 443 (HTTPS)                         │
│  - Port 6550 -> 6443 (Kubernetes API)               │
└─────────────────────────────────────────────────────┘
```

**Команда создания кластера:**

```bash
k3d cluster create minitoolstream \
  --servers 1 \
  --agents 2 \
  --port "8080:80@loadbalancer" \
  --port "8443:443@loadbalancer" \
  --api-port 6550 \
  --volume "/Users/moroshma/go/DiplomaThesis/MiniToolStream:/workspace@all" \
  --registry-create minitoolstream-registry:5000
```

**Параметры:**
- `--servers 1` — 1 control plane нода (master)
- `--agents 2` — 2 worker ноды
- `--port` — проброс портов для доступа к сервисам
- `--api-port 6550` — порт API сервера (вместо стандартного 6443)
- `--volume` — монтирование локальной директории в контейнеры
- `--registry-create` — локальный Docker registry для образов

**Проверка кластера:**

```bash
# Проверка нод
kubectl get nodes

# Ожидаемый вывод:
# NAME                          STATUS   ROLES                  AGE   VERSION
# k3d-minitoolstream-server-0   Ready    control-plane,master   1m    v1.33.4+k3s1
# k3d-minitoolstream-agent-0    Ready    <none>                 1m    v1.33.4+k3s1
# k3d-minitoolstream-agent-1    Ready    <none>                 1m    v1.33.4+k3s1

# Информация о кластере
kubectl cluster-info

# Kubernetes control plane is running at https://0.0.0.0:6550
# CoreDNS is running at https://0.0.0.0:6550/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
# Metrics-server is running at https://0.0.0.0:6550/api/v1/namespaces/kube-system/services/https:metrics-server:https/proxy

# Проверка всех подов системы
kubectl get pods --all-namespaces
```

### 5.1.4 Настройка kubectl

**Автоматическая настройка:**

k3d автоматически обновляет `~/.kube/config` с контекстом нового кластера.

```bash
# Проверка текущего контекста
kubectl config current-context
# k3d-minitoolstream

# Список всех контекстов
kubectl config get-contexts

# Переключение контекста (если нужно)
kubectl config use-context k3d-minitoolstream
```

**Ручная настройка (если требуется):**

```bash
# Экспорт kubeconfig
k3d kubeconfig get minitoolstream > ~/.kube/k3d-minitoolstream.config

# Использование
export KUBECONFIG=~/.kube/k3d-minitoolstream.config
kubectl get nodes
```

### 5.1.5 Создание namespaces

Kubernetes namespace обеспечивает логическую изоляцию ресурсов.

**Структура namespaces для MiniToolStream:**

```bash
# Создание основного namespace
kubectl create namespace minitoolstream

# Создание namespace для мониторинга
kubectl create namespace monitoring

# Просмотр всех namespaces
kubectl get namespaces

# NAME              STATUS   AGE
# default           Active   5m
# kube-system       Active   5m
# kube-public       Active   5m
# kube-node-lease   Active   5m
# minitoolstream    Active   1m
# monitoring        Active   30s
```

**Установка namespace по умолчанию:**

```bash
# Для текущей сессии
kubectl config set-context --current --namespace=minitoolstream

# Проверка
kubectl config view --minify | grep namespace:
```

**Альтернативный способ через YAML:**

`namespaces.yaml`:
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: minitoolstream
  labels:
    name: minitoolstream
    environment: development
---
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring
  labels:
    name: monitoring
    purpose: observability
```

```bash
kubectl apply -f namespaces.yaml
```

### 5.1.6 Установка необходимых компонентов

**1. Metrics Server (для HPA)**

k3s включает metrics-server по умолчанию, но проверим:

```bash
# Проверка metrics-server
kubectl get deployment metrics-server -n kube-system

# Проверка метрик нод
kubectl top nodes

# NAME                          CPU(cores)   CPU%   MEMORY(bytes)   MEMORY%
# k3d-minitoolstream-server-0   245m         6%     1024Mi          25%
# k3d-minitoolstream-agent-0    112m         2%     768Mi           19%
# k3d-minitoolstream-agent-1    98m          2%     654Mi           16%

# Проверка метрик подов
kubectl top pods -n kube-system
```

Если metrics-server не установлен:

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

**2. Kubernetes Dashboard**

Для визуального управления кластером:

```bash
# Установка Dashboard
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

# Создание admin пользователя
cat <<EOF | kubectl apply -f -
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
EOF

# Генерация токена доступа
kubectl -n kubernetes-dashboard create token admin-user --duration=87600h

# Сохранение токена
kubectl -n kubernetes-dashboard create token admin-user --duration=87600h > k3s-dashboard-token.txt

# Запуск proxy для доступа
kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard-kong-proxy 8443:443

# Открыть в браузере
open https://localhost:8443
```

**3. Локальный Docker Registry**

k3d создает registry автоматически, но можно создать вручную:

```bash
# Создание registry
k3d registry create minitoolstream-registry.localhost --port 5000

# Подключение к кластеру
k3d cluster create minitoolstream --registry-use k3d-minitoolstream-registry:5000
```

**Использование registry:**

```bash
# Тегирование образа
docker tag minitoolstream-ingress:latest k3d-minitoolstream-registry:5000/minitoolstream-ingress:latest

# Загрузка в registry
docker push k3d-minitoolstream-registry:5000/minitoolstream-ingress:latest

# Импорт образа напрямую в k3d (быстрее)
k3d image import minitoolstream-ingress:latest -c minitoolstream
```

### 5.1.7 Подготовка секретов и конфигураций

**Создание секретов для MiniToolStream:**

```bash
# Секреты для Ingress сервиса
kubectl create secret generic minitoolstream-ingress-secret \
  -n minitoolstream \
  --from-literal=TARANTOOL_PASSWORD=supersecret \
  --from-literal=MINIO_ACCESS_KEY_ID=minioadmin \
  --from-literal=MINIO_SECRET_ACCESS_KEY=minioadmin

# Секреты для Egress сервиса
kubectl create secret generic minitoolstream-egress-secret \
  -n minitoolstream \
  --from-literal=TARANTOOL_PASSWORD=supersecret \
  --from-literal=MINIO_ACCESS_KEY_ID=minioadmin \
  --from-literal=MINIO_SECRET_ACCESS_KEY=minioadmin

# Проверка секретов
kubectl get secrets -n minitoolstream
```

**Альтернативный способ через YAML (для version control):**

`secrets.yaml`:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: minitoolstream-ingress-secret
  namespace: minitoolstream
type: Opaque
stringData:
  TARANTOOL_PASSWORD: supersecret
  MINIO_ACCESS_KEY_ID: minioadmin
  MINIO_SECRET_ACCESS_KEY: minioadmin
```

```bash
kubectl apply -f secrets.yaml
```

**Важно:** Секреты в base64 в YAML не обеспечивают реальной безопасности. Для production используйте:
- **Sealed Secrets** (Bitnami)
- **External Secrets Operator**
- **HashiCorp Vault**
- **AWS Secrets Manager / Azure Key Vault / GCP Secret Manager**

### 5.1.8 Подготовка Persistent Volumes

Для хранилищ данных (Tarantool, MinIO) нужны постоянные тома.

**StorageClass:**

k3s включает `local-path` StorageClass по умолчанию:

```bash
# Проверка StorageClass
kubectl get storageclass

# NAME                   PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE
# local-path (default)   rancher.io/local-path   Delete          WaitForFirstConsumer
```

**Пример PersistentVolumeClaim:**

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tarantool-data
  namespace: minitoolstream
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: local-path
  resources:
    requests:
      storage: 10Gi
```

```bash
kubectl apply -f pvc.yaml
kubectl get pvc -n minitoolstream
```

### 5.1.9 RBAC настройка

Создание ServiceAccount с минимальными правами для подов MiniToolStream:

`rbac.yaml`:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: minitoolstream-ingress
  namespace: minitoolstream
  labels:
    app: minitoolstream-ingress
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: minitoolstream-ingress-role
  namespace: minitoolstream
rules:
  # Разрешение читать ConfigMaps
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["get", "list", "watch"]
  # Разрешение читать Secrets
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
  # Разрешение читать информацию о подах (для service discovery)
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: minitoolstream-ingress-rolebinding
  namespace: minitoolstream
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: minitoolstream-ingress-role
subjects:
  - kind: ServiceAccount
    name: minitoolstream-ingress
    namespace: minitoolstream
```

```bash
kubectl apply -f rbac.yaml

# Проверка
kubectl get serviceaccount -n minitoolstream
kubectl get role -n minitoolstream
kubectl get rolebinding -n minitoolstream
```

### 5.1.10 Проверка готовности инфраструктуры

**Чеклист готовности:**

```bash
# 1. Кластер работает
kubectl get nodes
# Все ноды в статусе Ready

# 2. Namespaces созданы
kubectl get namespaces
# minitoolstream, monitoring присутствуют

# 3. Metrics доступны
kubectl top nodes
# Метрики отображаются

# 4. Секреты созданы
kubectl get secrets -n minitoolstream
# minitoolstream-ingress-secret, minitoolstream-egress-secret

# 5. RBAC настроен
kubectl get serviceaccount -n minitoolstream
# minitoolstream-ingress

# 6. StorageClass доступен
kubectl get storageclass
# local-path (default)

# 7. Dashboard доступен (опционально)
kubectl get pods -n kubernetes-dashboard
# Все поды Running
```

**Скрипт проверки:**

```bash
#!/bin/bash
# k3s-cluster-info.sh

echo "========================================="
echo "k3d-minitoolstream Cluster Information"
echo "========================================="
echo

echo "Cluster nodes:"
kubectl get nodes
echo

echo "Namespaces:"
kubectl get namespaces
echo

echo "Metrics Server:"
kubectl top nodes
echo

echo "Storage Classes:"
kubectl get storageclass
echo

echo "Secrets in minitoolstream namespace:"
kubectl get secrets -n minitoolstream
echo

echo "ServiceAccounts in minitoolstream namespace:"
kubectl get serviceaccount -n minitoolstream
echo

echo "Ready for deployment!"
```

```bash
chmod +x k3s-cluster-info.sh
./k3s-cluster-info.sh
```

---

## 5.2 Сценарий развертывания Helm

### 5.2.1 Введение в Helm

**Helm** — пакетный менеджер для Kubernetes, позволяющий упаковывать, конфигурировать и развертывать приложения в виде charts.

**Преимущества Helm:**

1. **Управление зависимостями** — автоматическая установка зависимых компонентов
2. **Шаблонизация** — параметризация манифестов через `values.yaml`
3. **Версионирование** — откат к предыдущим версиям (`helm rollback`)
4. **Переиспользование** — публикация charts в репозитории
5. **Упрощенное обновление** — `helm upgrade` вместо множества `kubectl apply`

### 5.2.2 Установка Helm

```bash
# macOS
brew install helm

# Linux
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Проверка версии
helm version

# version.BuildInfo{Version:"v3.14.0", GitCommit:"3fc9f4b2638", GitTreeState:"clean", GoVersion:"go1.21.5"}
```

### 5.2.3 Структура Helm Chart для MiniToolStream

Хотя в текущей реализации используются прямые Kubernetes манифесты, рекомендуемая структура Helm chart:

```
minitoolstream-helm/
├── Chart.yaml              # Метаданные chart
├── values.yaml             # Значения по умолчанию
├── values-production.yaml  # Значения для production
├── values-development.yaml # Значения для development
├── templates/
│   ├── _helpers.tpl        # Шаблонные функции
│   ├── namespace.yaml
│   ├── ingress/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── hpa.yaml
│   │   ├── pdb.yaml
│   │   └── rbac.yaml
│   ├── egress/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── hpa.yaml
│   │   └── rbac.yaml
│   ├── tarantool/
│   │   ├── statefulset.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   └── pvc.yaml
│   └── minio/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── configmap.yaml
│       ├── secret.yaml
│       └── pvc.yaml
├── charts/                 # Зависимые charts (sub-charts)
│   └── (пусто или external dependencies)
└── README.md
```

### 5.2.4 Chart.yaml

`Chart.yaml`:
```yaml
apiVersion: v2
name: minitoolstream
description: A high-performance message streaming system with object storage
type: application
version: 1.0.0
appVersion: "1.0.0"

keywords:
  - messaging
  - streaming
  - grpc
  - object-storage

home: https://github.com/moroshma/MiniToolStream
sources:
  - https://github.com/moroshma/MiniToolStream

maintainers:
  - name: moroshma
    email: moroshma@example.com

dependencies: []
```

### 5.2.5 values.yaml (основной конфигурационный файл)

`values.yaml`:
```yaml
# Global settings
global:
  namespace: minitoolstream
  imagePullPolicy: IfNotPresent
  storageClass: local-path

# Ingress Service
ingress:
  enabled: true
  name: minitoolstream-ingress
  replicaCount: 3

  image:
    repository: minitoolstream-ingress
    tag: latest
    pullPolicy: IfNotPresent

  service:
    type: NodePort
    port: 50051
    nodePort: 30051
    sessionAffinity: ClientIP
    sessionAffinityTimeout: 10800

  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "512Mi"
      cpu: "500m"

  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80

  podDisruptionBudget:
    enabled: true
    minAvailable: 2

  config:
    server:
      port: 50051
    logger:
      level: info
      format: json
    ttl:
      enabled: true
      default: 24h

  secrets:
    tarantoolPassword: supersecret
    minioAccessKeyId: minioadmin
    minioSecretAccessKey: minioadmin

# Egress Service
egress:
  enabled: true
  name: minitoolstream-egress
  replicaCount: 3

  image:
    repository: minitoolstream-egress
    tag: latest
    pullPolicy: IfNotPresent

  service:
    type: NodePort
    port: 50052
    nodePort: 30052

  resources:
    requests:
      memory: "256Mi"
      cpu: "250m"
    limits:
      memory: "512Mi"
      cpu: "500m"

  autoscaling:
    enabled: true
    minReplicas: 3
    maxReplicas: 10
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80

# Tarantool
tarantool:
  enabled: true
  name: tarantool

  image:
    repository: tarantool/tarantool
    tag: "2.11"

  service:
    type: ClusterIP
    port: 3301

  persistence:
    enabled: true
    size: 10Gi
    storageClass: local-path

  resources:
    requests:
      memory: "512Mi"
      cpu: "500m"
    limits:
      memory: "1Gi"
      cpu: "1000m"

# MinIO
minio:
  enabled: true
  name: minio

  image:
    repository: minio/minio
    tag: latest

  service:
    type: ClusterIP
    port: 9000
    consolePort: 9001

  persistence:
    enabled: true
    size: 50Gi
    storageClass: local-path

  resources:
    requests:
      memory: "1Gi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "1000m"

  secrets:
    rootUser: minioadmin
    rootPassword: minioadmin

# Vault (optional)
vault:
  enabled: false
```

### 5.2.6 Шаблонизация манифестов

**Пример: `templates/ingress/deployment.yaml`**

```yaml
{{- if .Values.ingress.enabled }}
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.ingress.name }}
  namespace: {{ .Values.global.namespace }}
  labels:
    app: {{ .Values.ingress.name }}
    chart: {{ .Chart.Name }}-{{ .Chart.Version }}
    release: {{ .Release.Name }}
    heritage: {{ .Release.Service }}
spec:
  replicas: {{ .Values.ingress.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Values.ingress.name }}
  template:
    metadata:
      labels:
        app: {{ .Values.ingress.name }}
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "{{ .Values.ingress.service.port }}"
    spec:
      serviceAccountName: {{ .Values.ingress.name }}
      containers:
        - name: ingress
          image: "{{ .Values.ingress.image.repository }}:{{ .Values.ingress.image.tag }}"
          imagePullPolicy: {{ .Values.ingress.image.pullPolicy }}
          ports:
            - name: grpc
              containerPort: {{ .Values.ingress.config.server.port }}
          env:
            - name: SERVER_PORT
              value: "{{ .Values.ingress.config.server.port }}"
            - name: TARANTOOL_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ .Values.ingress.name }}-secret
                  key: TARANTOOL_PASSWORD
          resources:
            {{- toYaml .Values.ingress.resources | nindent 12 }}
{{- end }}
```

### 5.2.7 Использование _helpers.tpl

`templates/_helpers.tpl`:
```yaml
{{/*
Expand the name of the chart.
*/}}
{{- define "minitoolstream.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "minitoolstream.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "minitoolstream.labels" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
app.kubernetes.io/name: {{ include "minitoolstream.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}
```

### 5.2.8 Сценарий установки через Helm

**Шаг 1: Валидация chart**

```bash
cd minitoolstream-helm

# Проверка синтаксиса
helm lint .

# Dry-run для проверки генерируемых манифестов
helm install minitoolstream . \
  --dry-run \
  --debug \
  --namespace minitoolstream

# Шаблонизация без установки (для просмотра результата)
helm template minitoolstream . \
  --namespace minitoolstream \
  --values values.yaml > rendered-manifests.yaml
```

**Шаг 2: Установка с значениями по умолчанию**

```bash
# Создание namespace (если еще не создан)
kubectl create namespace minitoolstream

# Установка chart
helm install minitoolstream . \
  --namespace minitoolstream \
  --create-namespace

# Вывод:
# NAME: minitoolstream
# LAST DEPLOYED: Tue Dec 17 18:00:00 2025
# NAMESPACE: minitoolstream
# STATUS: deployed
# REVISION: 1
# NOTES:
# MiniToolStream has been installed!
#
# Access the services:
# - Ingress: kubectl port-forward svc/minitoolstream-ingress-service 50051:50051 -n minitoolstream
# - Egress: kubectl port-forward svc/minitoolstream-egress-service 50052:50052 -n minitoolstream
```

**Шаг 3: Проверка установки**

```bash
# Статус release
helm status minitoolstream -n minitoolstream

# Список всех releases
helm list -n minitoolstream

# Проверка подов
kubectl get pods -n minitoolstream

# Проверка сервисов
kubectl get services -n minitoolstream

# Логи
kubectl logs -l app=minitoolstream-ingress -n minitoolstream --tail=50
```

**Шаг 4: Установка с кастомными значениями**

```bash
# Используя файл values
helm install minitoolstream . \
  --namespace minitoolstream \
  --values values-production.yaml

# Переопределение конкретных значений через --set
helm install minitoolstream . \
  --namespace minitoolstream \
  --set ingress.replicaCount=5 \
  --set ingress.resources.limits.memory=1Gi \
  --set ingress.autoscaling.maxReplicas=20
```

### 5.2.9 Управление через Helm

**Обновление релиза:**

```bash
# Изменить values.yaml или создать новый файл

# Обновление с новыми значениями
helm upgrade minitoolstream . \
  --namespace minitoolstream \
  --values values.yaml \
  --set ingress.image.tag=v1.1.0

# С автоматической установкой, если релиза нет
helm upgrade --install minitoolstream . \
  --namespace minitoolstream \
  --values values.yaml
```

**Откат к предыдущей версии:**

```bash
# Просмотр истории релизов
helm history minitoolstream -n minitoolstream

# REVISION  UPDATED                   STATUS      CHART                 DESCRIPTION
# 1         Tue Dec 17 18:00:00 2025  superseded  minitoolstream-1.0.0  Install complete
# 2         Tue Dec 17 18:30:00 2025  superseded  minitoolstream-1.0.0  Upgrade complete
# 3         Tue Dec 17 19:00:00 2025  deployed    minitoolstream-1.0.0  Upgrade complete

# Откат к ревизии 2
helm rollback minitoolstream 2 -n minitoolstream

# Откат к предыдущей версии
helm rollback minitoolstream -n minitoolstream
```

**Удаление релиза:**

```bash
# Удаление (с сохранением истории)
helm uninstall minitoolstream -n minitoolstream

# Полное удаление с историей
helm uninstall minitoolstream -n minitoolstream --no-hooks

# Удаление namespace
kubectl delete namespace minitoolstream
```

### 5.2.10 Альтернативный подход: kubectl + kustomize

Если Helm избыточен, можно использовать **kustomize** — встроенный в kubectl инструмент для кастомизации манифестов.

**Структура с kustomize:**

```
k8s/
├── base/
│   ├── kustomization.yaml
│   ├── namespace.yaml
│   ├── ingress/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   └── configmap.yaml
│   └── tarantool/
│       └── statefulset.yaml
└── overlays/
    ├── development/
    │   ├── kustomization.yaml
    │   └── patches/
    │       └── ingress-replicas.yaml
    └── production/
        ├── kustomization.yaml
        └── patches/
            └── ingress-resources.yaml
```

**base/kustomization.yaml:**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: minitoolstream

resources:
  - namespace.yaml
  - ingress/deployment.yaml
  - ingress/service.yaml
  - ingress/configmap.yaml
  - tarantool/statefulset.yaml

commonLabels:
  app: minitoolstream
  managed-by: kustomize
```

**overlays/production/kustomization.yaml:**

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - ../../base

patchesStrategicMerge:
  - patches/ingress-replicas.yaml
  - patches/ingress-resources.yaml

replicas:
  - name: minitoolstream-ingress
    count: 5
```

**Применение:**

```bash
# Development
kubectl apply -k k8s/overlays/development

# Production
kubectl apply -k k8s/overlays/production

# Просмотр без применения
kubectl kustomize k8s/overlays/production
```

---

## 5.3 Настройка мониторинга (Prometheus + Grafana)

### 5.3.1 Архитектура мониторинга в Kubernetes

```
┌────────────────────────────────────────────────────────┐
│             Kubernetes Cluster (k3d)                   │
│                                                        │
│  ┌─────────────────────────────────────────────────┐  │
│  │     Namespace: minitoolstream                    │  │
│  │                                                  │  │
│  │  ┌──────────────┐  ┌──────────────┐            │  │
│  │  │   Ingress    │  │   Egress     │            │  │
│  │  │   Pods (3)   │  │   Pods (3)   │            │  │
│  │  └──────┬───────┘  └──────┬───────┘            │  │
│  │         │ metrics          │ metrics            │  │
│  │         └──────────┬───────┘                    │  │
│  └────────────────────┼──────────────────────────┘  │
│                       │                              │
│  ┌────────────────────▼──────────────────────────┐  │
│  │     Namespace: monitoring                     │  │
│  │                                               │  │
│  │  ┌─────────────────────────────────────────┐ │  │
│  │  │        Prometheus Operator              │ │  │
│  │  │  ┌───────────────────────────────────┐  │ │  │
│  │  │  │      Prometheus Server            │  │ │  │
│  │  │  │  • ServiceMonitor discovery       │  │ │  │
│  │  │  │  • Metrics scraping (15s)         │  │ │  │
│  │  │  │  • TSDB storage                   │  │ │  │
│  │  │  │  • AlertManager integration       │  │ │  │
│  │  │  └────────────┬──────────────────────┘  │ │  │
│  │  └───────────────┼──────────────────────────┘ │  │
│  │                  │                            │  │
│  │  ┌───────────────▼──────────────────────────┐ │  │
│  │  │            Grafana                       │ │  │
│  │  │  • Dashboards (pre-configured)           │ │  │
│  │  │  • PromQL queries                        │ │  │
│  │  │  • Alerting                              │ │  │
│  │  │  • User management                       │ │  │
│  │  └──────────────────────────────────────────┘ │  │
│  │                                               │  │
│  │  ┌──────────────────────────────────────────┐ │  │
│  │  │         kube-state-metrics               │ │  │
│  │  │  (Kubernetes objects metrics)            │ │  │
│  │  └──────────────────────────────────────────┘ │  │
│  │                                               │  │
│  │  ┌──────────────────────────────────────────┐ │  │
│  │  │          node-exporter                   │ │  │
│  │  │  (Host system metrics)                   │ │  │
│  │  └──────────────────────────────────────────┘ │  │
│  └───────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────┘
```

### 5.3.2 Установка kube-prometheus-stack через Helm

**kube-prometheus-stack** — это комплексное решение, включающее:
- Prometheus Operator
- Prometheus Server
- Alertmanager
- Grafana
- kube-state-metrics
- node-exporter
- Pre-configured dashboards

**Шаг 1: Добавление Helm репозитория**

```bash
# Добавление prometheus-community repo
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts

# Обновление repo
helm repo update

# Проверка
helm search repo prometheus-community/kube-prometheus-stack
```

**Шаг 2: Создание values для кастомизации**

`monitoring-values.yaml`:
```yaml
# Prometheus Operator
prometheusOperator:
  enabled: true

# Prometheus Server
prometheus:
  enabled: true
  prometheusSpec:
    retention: 30d
    retentionSize: "50GB"
    storageSpec:
      volumeClaimTemplate:
        spec:
          storageClassName: local-path
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 50Gi

    # ServiceMonitor для автодискавери
    serviceMonitorSelectorNilUsesHelmValues: false
    podMonitorSelectorNilUsesHelmValues: false

    # Resources
    resources:
      requests:
        cpu: 500m
        memory: 2Gi
      limits:
        cpu: 2000m
        memory: 4Gi

# Grafana
grafana:
  enabled: true
  adminPassword: admin

  persistence:
    enabled: true
    storageClassName: local-path
    size: 10Gi

  # Ingress (опционально)
  ingress:
    enabled: true
    hosts:
      - grafana.local

  # Pre-configured datasources
  datasources:
    datasources.yaml:
      apiVersion: 1
      datasources:
        - name: Prometheus
          type: prometheus
          url: http://prometheus-operated:9090
          access: proxy
          isDefault: true

  # Dashboards
  dashboardProviders:
    dashboardproviders.yaml:
      apiVersion: 1
      providers:
        - name: 'default'
          orgId: 1
          folder: ''
          type: file
          disableDeletion: false
          editable: true
          options:
            path: /var/lib/grafana/dashboards/default

  dashboards:
    default:
      # Kubernetes cluster monitoring
      kubernetes-cluster:
        gnetId: 7249
        revision: 1
        datasource: Prometheus

      # Node exporter
      node-exporter:
        gnetId: 1860
        revision: 27
        datasource: Prometheus

# AlertManager
alertmanager:
  enabled: true
  alertmanagerSpec:
    storage:
      volumeClaimTemplate:
        spec:
          storageClassName: local-path
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 10Gi

# kube-state-metrics
kube-state-metrics:
  enabled: true

# node-exporter
nodeExporter:
  enabled: true

# Default rules
defaultRules:
  create: true
  rules:
    alertmanager: true
    etcd: false  # k3s uses SQLite
    kubeApiserver: true
    kubeScheduler: true
    kubeStateMetrics: true
    kubelet: true
    kubernetesSystem: true
    node: true
```

**Шаг 3: Установка**

```bash
# Создание namespace
kubectl create namespace monitoring

# Установка kube-prometheus-stack
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --values monitoring-values.yaml

# Проверка установки
kubectl get pods -n monitoring

# Ожидаемый вывод:
# NAME                                                   READY   STATUS    AGE
# prometheus-kube-prometheus-operator-xxx                1/1     Running   1m
# prometheus-kube-state-metrics-xxx                      1/1     Running   1m
# prometheus-prometheus-node-exporter-xxx                1/1     Running   1m
# prometheus-grafana-xxx                                 3/3     Running   1m
# alertmanager-prometheus-kube-prometheus-alertmanager-0 2/2     Running   1m
# prometheus-prometheus-kube-prometheus-prometheus-0     2/2     Running   1m
```

### 5.3.3 Настройка ServiceMonitor для MiniToolStream

**ServiceMonitor** — CRD (Custom Resource Definition) от Prometheus Operator для автоматического обнаружения целей мониторинга.

`minitoolstream-servicemonitor.yaml`:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: minitoolstream-ingress
  namespace: minitoolstream
  labels:
    app: minitoolstream-ingress
    release: prometheus  # Важно для фильтрации Prometheus Operator
spec:
  selector:
    matchLabels:
      app: minitoolstream-ingress
  endpoints:
    - port: grpc
      interval: 15s
      path: /metrics
      scheme: http
  namespaceSelector:
    matchNames:
      - minitoolstream
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: minitoolstream-egress
  namespace: minitoolstream
  labels:
    app: minitoolstream-egress
    release: prometheus
spec:
  selector:
    matchLabels:
      app: minitoolstream-egress
  endpoints:
    - port: grpc
      interval: 15s
      path: /metrics
      scheme: http
  namespaceSelector:
    matchNames:
      - minitoolstream
```

```bash
kubectl apply -f minitoolstream-servicemonitor.yaml

# Проверка
kubectl get servicemonitor -n minitoolstream
```

### 5.3.4 Экспорт метрик из приложения

В Go-приложении (Ingress/Egress) нужно добавить Prometheus client:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    "net/http"
)

var (
    messagesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "minitoolstream_messages_total",
            Help: "Total number of messages processed",
        },
        []string{"service", "subject", "status"},
    )

    latencyHistogram = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "minitoolstream_latency_seconds",
            Help:    "Latency distribution",
            Buckets: prometheus.DefBuckets,
        },
        []string{"service", "operation"},
    )
)

func init() {
    prometheus.MustRegister(messagesTotal)
    prometheus.MustRegister(latencyHistogram)
}

func startMetricsServer() {
    http.Handle("/metrics", promhttp.Handler())
    log.Fatal(http.ListenAndServe(":50051", nil))  // Тот же порт что и gRPC, но другой handler
}
```

### 5.3.5 Доступ к Grafana

**Вариант 1: Port-forward**

```bash
kubectl port-forward -n monitoring svc/prometheus-grafana 3000:80

# Открыть в браузере
open http://localhost:3000

# Логин: admin
# Пароль: admin (или из monitoring-values.yaml)
```

**Вариант 2: NodePort**

Изменить тип сервиса:

```bash
kubectl patch svc prometheus-grafana -n monitoring -p '{"spec":{"type":"NodePort"}}'

# Получить порт
kubectl get svc prometheus-grafana -n monitoring

# Доступ через NodePort
open http://localhost:<node-port>
```

**Вариант 3: Ingress**

Если включен Ingress в values:

```yaml
grafana:
  ingress:
    enabled: true
    hosts:
      - grafana.local
```

Добавить в `/etc/hosts`:
```
127.0.0.1 grafana.local
```

Доступ: `http://grafana.local:8080`

### 5.3.6 Создание дашборда для MiniToolStream

**Dashboard JSON для импорта:**

```json
{
  "dashboard": {
    "title": "MiniToolStream Monitoring",
    "panels": [
      {
        "title": "Messages Throughput (msg/sec)",
        "targets": [
          {
            "expr": "rate(minitoolstream_messages_total{status=\"success\"}[1m])",
            "legendFormat": "{{service}} - {{subject}}"
          }
        ],
        "type": "graph"
      },
      {
        "title": "Latency P50/P95/P99",
        "targets": [
          {
            "expr": "histogram_quantile(0.50, rate(minitoolstream_latency_seconds_bucket[5m]))",
            "legendFormat": "P50"
          },
          {
            "expr": "histogram_quantile(0.95, rate(minitoolstream_latency_seconds_bucket[5m]))",
            "legendFormat": "P95"
          },
          {
            "expr": "histogram_quantile(0.99, rate(minitoolstream_latency_seconds_bucket[5m]))",
            "legendFormat": "P99"
          }
        ],
        "type": "graph"
      },
      {
        "title": "Error Rate",
        "targets": [
          {
            "expr": "rate(minitoolstream_messages_total{status=\"error\"}[1m])",
            "legendFormat": "{{service}} - {{subject}}"
          }
        ],
        "type": "graph"
      }
    ]
  }
}
```

**Импорт в Grafana:**

1. Открыть Grafana UI
2. Dashboards → Import
3. Вставить JSON или загрузить файл
4. Выбрать Prometheus datasource
5. Import

### 5.3.7 Alerts и уведомления

**PrometheusRule для алертов:**

`minitoolstream-alerts.yaml`:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: minitoolstream-alerts
  namespace: minitoolstream
  labels:
    prometheus: kube-prometheus
    role: alert-rules
spec:
  groups:
    - name: minitoolstream
      interval: 30s
      rules:
        - alert: HighErrorRate
          expr: |
            rate(minitoolstream_messages_total{status="error"}[5m]) > 0.05
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "High error rate detected"
            description: "Error rate is {{ $value | humanizePercentage }} for {{ $labels.service }}"

        - alert: HighLatency
          expr: |
            histogram_quantile(0.95, rate(minitoolstream_latency_seconds_bucket[5m])) > 0.5
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "High latency detected"
            description: "P95 latency is {{ $value }}s for {{ $labels.service }}"

        - alert: PodDown
          expr: |
            kube_pod_status_phase{namespace="minitoolstream", phase!="Running"} > 0
          for: 2m
          labels:
            severity: critical
          annotations:
            summary: "Pod {{ $labels.pod }} is down"
            description: "Pod {{ $labels.pod }} in namespace {{ $labels.namespace }} is not running"
```

```bash
kubectl apply -f minitoolstream-alerts.yaml

# Проверка
kubectl get prometheusrules -n minitoolstream
```

---

## 5.4 Возможности масштабирования (HPA)

### 5.4.1 Введение в HPA (Horizontal Pod Autoscaler)

**HPA** автоматически масштабирует количество подов на основе метрик:
- CPU utilization
- Memory utilization
- Custom metrics (из Prometheus)
- External metrics (из внешних систем)

**Принцип работы:**

```
┌─────────────────────────────────────────────────┐
│                                                 │
│  ┌───────────────────────────────────────────┐ │
│  │         HPA Controller                    │ │
│  │  1. Query metrics every 15s               │ │
│  │  2. Calculate desired replicas            │ │
│  │  3. Update Deployment                     │ │
│  └───────────┬───────────────────────────────┘ │
│              │                                  │
│              ▼                                  │
│  ┌───────────────────────────────────────────┐ │
│  │       Metrics Server / Prometheus         │ │
│  │  • CPU: 75% (target: 70%)                 │ │
│  │  • Memory: 85% (target: 80%)              │ │
│  └───────────┬───────────────────────────────┘ │
│              │                                  │
│              ▼                                  │
│  ┌───────────────────────────────────────────┐ │
│  │          Deployment                       │ │
│  │  Current: 3 replicas                      │ │
│  │  Desired: 4 replicas (scale up!)          │ │
│  └───────────────────────────────────────────┘ │
│                                                 │
└─────────────────────────────────────────────────┘
```

**Формула расчета:**

```
desiredReplicas = ceil[currentReplicas * (currentMetricValue / targetMetricValue)]
```

Пример:
- currentReplicas = 3
- currentCPU = 75%
- targetCPU = 70%
- desiredReplicas = ceil[3 * (75 / 70)] = ceil[3.21] = 4

### 5.4.2 HPA для MiniToolStream Ingress

`ingress-hpa.yaml`:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: minitoolstream-ingress-hpa
  namespace: minitoolstream
  labels:
    app: minitoolstream-ingress
    component: autoscaling
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: minitoolstream-ingress

  minReplicas: 3
  maxReplicas: 10

  # Метрики для масштабирования
  metrics:
    # CPU-based scaling
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70

    # Memory-based scaling
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80

  # Поведение масштабирования
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300  # Ждать 5 минут перед scale down
      policies:
        - type: Percent
          value: 50                     # Уменьшать максимум на 50% за раз
          periodSeconds: 60
        - type: Pods
          value: 2                      # Или максимум на 2 пода за раз
          periodSeconds: 60
      selectPolicy: Min                 # Выбрать более консервативную политику

    scaleUp:
      stabilizationWindowSeconds: 0     # Немедленно scale up
      policies:
        - type: Percent
          value: 100                    # Увеличивать до 100% (удвоить) за раз
          periodSeconds: 30
        - type: Pods
          value: 4                      # Или максимум на 4 пода за раз
          periodSeconds: 30
      selectPolicy: Max                 # Выбрать более агрессивную политику
```

**Применение:**

```bash
kubectl apply -f ingress-hpa.yaml

# Проверка HPA
kubectl get hpa -n minitoolstream

# NAME                          REFERENCE                         TARGETS         MINPODS   MAXPODS   REPLICAS   AGE
# minitoolstream-ingress-hpa    Deployment/minitoolstream-ingress 45%/70%, 60%/80%   3         10        3          1m

# Детальная информация
kubectl describe hpa minitoolstream-ingress-hpa -n minitoolstream
```

### 5.4.3 Custom Metrics Autoscaling

Масштабирование на основе кастомных метрик из Prometheus (например, количество сообщений в очереди):

**Требуется установка Prometheus Adapter:**

```bash
helm install prometheus-adapter prometheus-community/prometheus-adapter \
  --namespace monitoring \
  --set prometheus.url=http://prometheus-operated.monitoring.svc \
  --set prometheus.port=9090
```

**Конфигурация адаптера для кастомных метрик:**

`prometheus-adapter-values.yaml`:
```yaml
rules:
  default: false
  custom:
    - seriesQuery: 'minitoolstream_messages_total{status="success"}'
      resources:
        overrides:
          namespace: {resource: "namespace"}
          pod: {resource: "pod"}
      name:
        matches: "^(.*)_total$"
        as: "${1}_per_second"
      metricsQuery: 'rate(<<.Series>>{<<.LabelMatchers>>}[1m])'
```

```bash
helm upgrade prometheus-adapter prometheus-community/prometheus-adapter \
  --namespace monitoring \
  --values prometheus-adapter-values.yaml
```

**HPA с кастомной метрикой:**

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: minitoolstream-ingress-custom-hpa
  namespace: minitoolstream
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: minitoolstream-ingress
  minReplicas: 3
  maxReplicas: 20
  metrics:
    # CPU (как базовая метрика)
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70

    # Кастомная метрика: сообщения в секунду
    - type: Pods
      pods:
        metric:
          name: minitoolstream_messages_per_second
        target:
          type: AverageValue
          averageValue: "1000"  # 1000 msg/sec per pod
```

### 5.4.4 Vertical Pod Autoscaler (VPA)

**VPA** автоматически настраивает requests/limits для CPU и памяти.

**Установка VPA:**

```bash
git clone https://github.com/kubernetes/autoscaler.git
cd autoscaler/vertical-pod-autoscaler
./hack/vpa-up.sh

# Проверка
kubectl get pods -n kube-system | grep vpa
```

**VPA для Ingress:**

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: minitoolstream-ingress-vpa
  namespace: minitoolstream
spec:
  targetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: minitoolstream-ingress
  updatePolicy:
    updateMode: "Auto"  # "Off", "Initial", "Recreate", "Auto"
  resourcePolicy:
    containerPolicies:
      - containerName: ingress
        minAllowed:
          cpu: 100m
          memory: 128Mi
        maxAllowed:
          cpu: 2000m
          memory: 2Gi
        controlledResources: ["cpu", "memory"]
```

```bash
kubectl apply -f ingress-vpa.yaml

# Проверка рекомендаций
kubectl describe vpa minitoolstream-ingress-vpa -n minitoolstream
```

**Важно:** HPA и VPA могут конфликтовать при использовании CPU/Memory метрик одновременно. Рекомендуется:
- HPA для CPU, VPA для Memory
- Или только HPA с кастомными метриками

### 5.4.5 Cluster Autoscaler

**Cluster Autoscaler** масштабирует сами ноды кластера.

Для **k3d** это не применимо, так как ноды — это Docker-контейнеры. Но для облачных провайдеров:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cluster-autoscaler
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: cluster-autoscaler
  template:
    spec:
      containers:
        - name: cluster-autoscaler
          image: k8s.gcr.io/autoscaling/cluster-autoscaler:v1.28.0
          command:
            - ./cluster-autoscaler
            - --cloud-provider=aws  # или gce, azure
            - --nodes=1:10:worker-node-group
```

### 5.4.6 Тестирование автомасштабирования

**Сценарий нагрузочного теста:**

```bash
# 1. Проверка начального состояния
kubectl get pods -n minitoolstream
kubectl get hpa -n minitoolstream

# 2. Запуск генератора нагрузки
kubectl run -i --tty load-generator --rm --image=busybox --restart=Never -- /bin/sh

# В интерактивной сессии:
while true; do
  # Симуляция запросов к Ingress
  nc -z minitoolstream-ingress-service.minitoolstream.svc.cluster.local 50051
done

# 3. В другом терминале наблюдение за HPA
watch kubectl get hpa -n minitoolstream

# 4. Наблюдение за подами
watch kubectl get pods -n minitoolstream

# 5. Остановка нагрузки (Ctrl+C в load-generator)

# 6. Наблюдение за scale down (через 5 минут)
```

**Визуализация в Grafana:**

PromQL запросы для дашборда:

```promql
# Количество реплик
kube_deployment_status_replicas{namespace="minitoolstream", deployment="minitoolstream-ingress"}

# CPU utilization
rate(container_cpu_usage_seconds_total{namespace="minitoolstream", pod=~"minitoolstream-ingress-.*"}[1m]) * 100

# Memory utilization
container_memory_usage_bytes{namespace="minitoolstream", pod=~"minitoolstream-ingress-.*"} /
container_spec_memory_limit_bytes{namespace="minitoolstream", pod=~"minitoolstream-ingress-.*"} * 100

# HPA desired replicas
kube_horizontalpodautoscaler_status_desired_replicas{namespace="minitoolstream"}
```

### 5.4.7 Best Practices для масштабирования

**1. Настройка ресурсов:**

```yaml
resources:
  requests:
    cpu: 250m      # Гарантированный CPU
    memory: 256Mi  # Гарантированная память
  limits:
    cpu: 500m      # Максимальный CPU
    memory: 512Mi  # Максимальная память
```

- `requests` — для планирования и HPA
- `limits` — для защиты от runaway processes

**2. PodDisruptionBudget (PDB):**

Гарантия минимального количества подов во время обновлений:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: minitoolstream-ingress-pdb
  namespace: minitoolstream
spec:
  minAvailable: 2  # Или maxUnavailable: 1
  selector:
    matchLabels:
      app: minitoolstream-ingress
```

**3. Readiness и Liveness Probes:**

Убедиться, что HPA учитывает только готовые поды:

```yaml
readinessProbe:
  exec:
    command:
      - /bin/sh
      - -c
      - "nc -z localhost 50051"
  initialDelaySeconds: 10
  periodSeconds: 5
```

**4. Антиаффинити для распределения:**

Распределение подов по разным нодам:

```yaml
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app
                operator: In
                values:
                  - minitoolstream-ingress
          topologyKey: kubernetes.io/hostname
```

**5. Мониторинг и алерты:**

```yaml
- alert: HPAMaxedOut
  expr: |
    kube_horizontalpodautoscaler_status_current_replicas{namespace="minitoolstream"} >=
    kube_horizontalpodautoscaler_spec_max_replicas{namespace="minitoolstream"}
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "HPA {{ $labels.horizontalpodautoscaler }} has reached max replicas"
```

---

## Выводы

Развертывание MiniToolStream в Kubernetes обеспечивает:

1. **Высокую доступность** — через реплики и PDB
2. **Автоматическое масштабирование** — HPA на основе CPU, памяти и кастомных метрик
3. **Мониторинг в реальном времени** — Prometheus + Grafana с pre-configured dashboards
4. **Упрощенное управление** — через Helm или kubectl + kustomize
5. **Изоляцию и безопасность** — через namespaces, RBAC, secrets
6. **Production-ready инфраструктуру** — с логированием, алертами и автовосстановлением

Данная конфигурация позволяет легко мигрировать из локального k3d-кластера в облачный Kubernetes (GKE, EKS, AKS) с минимальными изменениями.
