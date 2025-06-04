
# Projeto API Gateway com Templates e Health Service

Este README explica em detalhes como configurar e rodar:
1. A **API de Health** (FastAPI em Python).
2. O **Gateway** em Go que faz proxy reverso e renderiza templates.
3. Os **templates** internos servidos pelo Gateway.
4. Configuração de **logs** por rota.
5. Arquivo `config.yml` com opções de rota.
6. Orquestração com **Docker Compose**.

---

## Estrutura de Pastas

```
GoLang/
│   docker-compose.yml
│   README.md            
│
├───gateway
│       config.yml       <- Configura rotas e templates
│       Dockerfile       <- Dockerfile do Gateway
│       go.mod           <- Módulos Go
│       go.sum
│       main.go          <- Código principal do Gateway
│
├───logs
│   └───gateway
│       ├───health
│       │       health.log
│       │
│       ├───template_example
│       │       template_example.log
│       │
│       └───template_health
│               template_health.log
│
├───services
│   └───health_service
│           app.py       <- FastAPI: rota /health
│           Dockerfile   <- Dockerfile do Health Service
│           requirements.txt
│
└───templates
    └───template_health
            index.html   <- HTML para rota /template_health
```

---

## 1. API de Saúde (Health Service)

### 1.1. Como criar

1. Crie a pasta `services/health_service`.
2. Dentro dela, crie o arquivo `app.py` com o conteúdo:

   ```python
   from fastapi import FastAPI

   app = FastAPI()

   @app.get("/health")
   async def health():
       return {"status": "ok"}
   ```

3. Crie `requirements.txt` com:

   ```
   fastapi
   hypercorn
   uvicorn
   uvloop
   ```

### 1.2. Dockerfile

Dentro de `services/health_service/Dockerfile`, adicione:

```dockerfile
# services/health_service/Dockerfile

FROM python:3.11-slim

WORKDIR /app

# Copia dependências e instala
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copia o código da aplicação
COPY app.py .

# Expõe porta 8000 para o container
EXPOSE 8000

# Inicia servidor ASGI com Hypercorn
CMD ["hypercorn", "app:app", "--bind", "0.0.0.0:8000"]
```

### 1.3. Testando a API de Health

Na raiz do projeto, execute:

```bash
cd services/health_service
docker build -t golang-health_service .
docker run -d --name health_service -p 8000:8000 golang-health_service
```

No navegador ou terminal:

```bash
curl http://localhost:8000/health
```

Deve retornar:

```json
{"status":"ok"}
```

---

## 2. Gateway em Go

O Gateway faz:
- Proxy reverso para serviços (como a rota `/health`).
- Renderiza templates internos (rota `/template_health`).

### 2.1. Como criar

1. Crie a pasta `gateway`.
2. Dentro dela, crie `go.mod` com:

   ```go
   module github.com/yourrepo/gateway

   go 1.24

   require (
       golang.org/x/net v0.18.0
       gopkg.in/yaml.v3 v3.0.1
   )
   ```

3. Rode `go mod tidy` para gerar `go.sum`.

4. Crie `main.go` com o código do Gateway (ver exemplo abaixo).

5. Crie `config.yml` para definir rotas e templates (ver item 3).

6. Crie `Dockerfile` para compilar o binário Go (ver item 4).

### 2.2. `main.go` (resumo)

No `gateway/main.go`, você deve implementar:

- Estruturas para carregar `config.yml` (leitura e parsing via `yaml.Unmarshal`).
- `newTemplateHandler(dir string)` que faz `template.ParseGlob(dir + "/*.html")`.
- Proxy reverso com `httputil.NewSingleHostReverseProxy`.
- Health check e balanceamento caso `target` tenha múltiplos backends.
- Middleware de logging (`createLoggedHandler`).
- Registro de rotas:
  - Se `TemplateDir` existir, usar `StripPrefix` para servir templates.
  - Caso contrário, registrar proxy diretamente.

**Exemplo simplificado de trecho de registro:**

```go
for _, svc := range cfg.Services {
    var handler http.Handler
    if svc.TemplateDir != "" {
        handler = newTemplateHandler(svc.TemplateDir)
        mux.Handle(svc.Route, http.StripPrefix(svc.Route, handler))
        mux.Handle(svc.Route+"/", http.StripPrefix(svc.Route+"/", handler))
    } else {
        handler = buildSingleHostProxy(svc.Target, transport)
        mux.Handle(svc.Route, handler)
        mux.Handle(svc.Route+"/", handler)
    }
}
```

---

## 3. Templates Internos

### 3.1. Como criar

1. Crie a pasta `templates/template_health`.
2. Dentro, crie `index.html`:

   ```html
   <!DOCTYPE html>
   <html lang="pt-BR">
   <head>
       <meta charset="UTF-8">
       <title>Template Health</title>
   </head>
   <body>
       <header>
           <h1>Rota Template Health</h1>
       </header>
       <main>
           <p>Esta página é servida diretamente pelo Gateway usando templates Go.</p>
       </main>
       <footer>
           <p>© 2025 Setup Tecnologia</p>
       </footer>
   </body>
   </html>
   ```

3. Ao acessar `http://localhost:8080/template_health/`, o Gateway:
   - Remove `/template_health` do path.
   - Carrega `index.html`.
   - Retorna o HTML.

### 3.2. Logs de Template

- O `config.yml` inclui:

  ```yaml
  - route: /template_health
    templateDir: /root/templates/template_health
    log: /var/log/gateway/template_health
  ```

- O Gateway cria `logs/gateway/template_health/template_health.log` e registra:

  ```
  [timestamp] <IP> GET /template_health/ -> 200 <latency>
  ```

---

## 4. Arquivo de Configuração (`gateway/config.yml`)

Exemplo completo:

```yaml
services:
  - route: /health
    target: http://health_service:8000
    log: /var/log/gateway/health

  - route: /template_health
    templateDir: /root/templates/template_health
    log: /var/log/gateway/template_health
```

### Opções:

- `route`: caminho HTTP no Gateway.
- `target`: URL do backend (proxy reverso).
- `templateDir`: pasta de templates (interna ao container).
- `log`: diretório para logs da rota.

---

## 5. Dockerfile do Gateway

No `gateway/Dockerfile`:

```dockerfile
# 1) Build stage
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o go_gateway .

# 2) Imagem final
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /root
COPY --from=builder /app/go_gateway .
COPY --from=builder /app/config.yml .
EXPOSE 80
CMD ["./go_gateway", "--config", "config.yml"]
```

- Multi-stage: compila em Go 1.24, depois copia binário para imagem leve.
- O `config.yml` é copiado para `/root/config.yml`.

---

## 6. Docker Compose

Arquivo `docker-compose.yml` na raiz:

```yaml
version: "3.8"

services:
  go_gateway:
    build:
      context: ./gateway
      dockerfile: Dockerfile
    image: golang-gateway:latest
    ports:
      - "8080:80"
    volumes:
      - ./templates:/root/templates
      - ./logs/gateway:/var/log/gateway
    depends_on:
      - health_service

  health_service:
    build:
      context: ./services/health_service
      dockerfile: Dockerfile
    image: golang-health_service:latest
    ports:
      - "8000:8000"
```

**Como criar e rodar:**

1. Na raiz (`GoLang/`), crie diretórios de log:
   ```bash
   mkdir -p logs/gateway/health logs/gateway/template_health
   ```
2. Construa e suba:
   ```bash
   docker-compose up --build
   ```

---

## 7. Rotas de Exemplo

- **/health**
  - URL: `http://localhost:8080/health`
  - Faz proxy para `http://health_service:8000/health` (FastAPI).
  - Log em `logs/gateway/health/health.log`.

- **/template_health**
  - URL: `http://localhost:8080/template_health/`
  - Remove prefixo e serve `templates/template_health/index.html`.
  - Log em `logs/gateway/template_health/template_health.log`.

---

## 8. Como Adicionar Novas Rotas

1. Edite `gateway/config.yml`:
   - Para proxy:
     ```yaml
     - route: /minharota
       target: http://outro_service:porta
       log: /var/log/gateway/minharota
     ```
   - Para template:
     ```yaml
     - route: /meutemplate
       templateDir: /root/templates/meutemplate
       log: /var/log/gateway/meutemplate
     ```
2. Coloque os arquivos HTML em `templates/meutemplate/*.html`.
3. Rode:
   ```bash
   docker-compose up --build
   ```
4. Acesse `http://localhost:8080/minharota/` ou `http://localhost:8080/meutemplate/`.

---
