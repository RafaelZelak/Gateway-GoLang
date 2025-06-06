# Guia Completo: Criando Serviços e Templates no API Gateway

Este documento explica passo a passo como criar **do zero** cada componente do projeto:
1. **API de Health** (exemplo em FastAPI).
2. **Serviço de Template** (exemplo em Go ou outro).
3. Como configurar o **gateway/config.yml** para registrar rotas.
4. Organização de pastas em **services** e **templates**.
5. Criar **Dockerfile** e **requirements.txt** para cada novo service.
6. Ajustar o **docker-compose.yml** para incluir novos containers.
7. Estrutura de **logs** por rota.

---

## 1. Estrutura Geral de Pastas

Antes de começar, monte sua estrutura básica em nível de projeto:

```
GoLang/
│   docker-compose.yml
│   README.md
│
├───gateway/
│       config.yml
│       Dockerfile
│       go.mod
│       go.sum
│       main.go
│
├───services/
│   ├───health_service/
│   │       app.py
│   │       Dockerfile
│   │       requirements.txt
│   │
│   └───<novo_service>/
│           main.<extensão>
│           Dockerfile
│           requirements.txt
│
├───templates/
│   ├───template_health/
│   │       index.html
│   │
│   └───<novo_template>/
│           index.html
│
└───logs/
    └───gateway/
        ├───health/
        │       health.log
        │
        └───template_health/
                template_health.log
```

- A pasta `gateway/` contém o código fonte e configuração do API Gateway em Go.
- A pasta `services/` contém cada microservice responsável por APIs novas (por exemplo, `health_service`).
- A pasta `templates/` contém diretórios com arquivos HTML para serviços de template internos.
- A pasta `logs/gateway/` armazena logs por rota (subpastas com o nome da rota).

---

## 2. Criando um Novo Serviço API (Health Service)

### 2.1. Criar a pasta e arquivos iniciais

```bash
cd GoLang/services
mkdir health_service
cd health_service
```

Dentro de **`services/health_service`**, crie:

1. **`app.py`** (exemplo com FastAPI):

   ```python
   from fastapi import FastAPI

   app = FastAPI()

   @app.get("/health")
   async def health():
       return {"status": "ok"}
   ```

2. **`requirements.txt`**:

   ```
   fastapi
   hypercorn
   uvicorn
   uvloop
   ```

3. **`Dockerfile`**:

   ```dockerfile
   # services/health_service/Dockerfile

   FROM python:3.11-slim

   WORKDIR /app

   # 1) Instala dependências
   COPY requirements.txt .
   RUN pip install --no-cache-dir -r requirements.txt

   # 2) Copia o código da aplicação
   COPY app.py .

   # 3) Expõe porta 8000
   EXPOSE 8000

   # 4) Comando para rodar o serviço
   CMD ["hypercorn", "app:app", "--bind", "0.0.0.0:8000"]
   ```

   - **Explicação**:
     1. Base `python:3.11-slim` para imagem leve.
     2. `pip install` instala `FastAPI`, `Hypercorn` e outros.
     3. O Hypercorn serve o app ASGI em `0.0.0.0:8000`.

### 2.2. Adicionar rota no `gateway/config.yml`

Abra **`gateway/config.yml`** e inclua:

```yaml
services:
  - route: /health
    target: http://health_service:8000
    log: /var/log/gateway/health
```

- **route**: `/health` é o caminho no Gateway.
- **target**: aponta para o container `health_service` na porta 8000.
- **log**: diretório onde o Gateway gravará logs dessa rota.

### 2.3. Adicionar serviço no `docker-compose.yml`

No arquivo **`docker-compose.yml`** na raiz, insira:

```yaml
version: "3.8"

services:
  go_gateway:
    # ... configurações do gateway ...
    depends_on:
      - health_service
    # volumes, ports etc.

  health_service:
    build:
      context: ./services/health_service
      dockerfile: Dockerfile
    image: golang-health_service:latest
    ports:
      - "8000:8000"
```

- Define que o serviço `health_service` é construido a partir da pasta `services/health_service`.
- Mapeia `8000` do host para `8000` do container.

---

## 3. Criando um Serviço de Template Interno

### 3.1. Criar pasta de template

```bash
cd GoLang/templates
mkdir template_health
cd template_health
```

Dentro de **`templates/template_health`**, crie **`index.html`**:

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

- O Go Gateway usará esse HTML para renderização na rota `/template_health`.

### 3.2. Adicionar rota no `gateway/config.yml`

Edite **`gateway/config.yml`** e adicione abaixo do `/health`:

```yaml
  - route: /template_health
    templateDir: /root/templates/template_health
    log: /var/log/gateway/template_health
```

- **route**: `/template_health` no Gateway.
- **templateDir**: caminho absoluto dentro do container (montagem de volume) onde estão os arquivos HTML.
- **log**: diretório para gravar logs dessa rota.

### 3.3. Atualizar `docker-compose.yml` para montar volume

No **`docker-compose.yml`**, em `go_gateway`:

```yaml
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
```

- Monta `./templates` do host em `/root/templates` no container:
  - Assim, `/root/templates/template_health/index.html` existe no container.
- Monta `./logs/gateway` para gravar logs do Gateway.

---

## 4. Como Funciona o Gateway (Go)

### 4.1. Arquivo `main.go`

No **`gateway/main.go`**, a lógica principal:

1. **Ler config.yml**:
   ```go
   data, _ := ioutil.ReadFile("config.yml")
   var cfg Config
   yaml.Unmarshal(data, &cfg)
   ```
2. **Criar transporte HTTP/2 (h2c)** para proxy com gRPC ou HTTP/2.
3. **Loop em `cfg.Services`**:
   ```go
   for _, svc := range cfg.Services {
       var handler http.Handler
       if svc.TemplateDir != "" {
           handler = newTemplateHandler(svc.TemplateDir)
       } else {
           handler = buildSingleHostProxy(svc.Target, transport)
       }
       if svc.Log != "" {
           handler = createLoggedHandler(handler, logger, routeName)
       }
       if svc.TemplateDir != "" {
           mux.Handle(svc.Route, http.StripPrefix(svc.Route, handler))
           mux.Handle(svc.Route+"/", http.StripPrefix(svc.Route+"/", handler))
       } else {
           mux.Handle(svc.Route, handler)
           mux.Handle(svc.Route+"/", handler)
       }
   }
   ```
   - **newTemplateHandler(dir)**: faz `template.ParseGlob(dir + "/*.html")` e guarda em memória.
   - **buildSingleHostProxy(target, transport)**: `httputil.NewSingleHostReverseProxy` para proxy reverso.
   - **http.StripPrefix**: remove a parte `/template_health` do path antes de executar o handler de template.
   - **createLoggedHandler**: envolve cada handler para gravar logs em `svc.Log`.

4. **Middleware de Logging**:
   ```go
   func createLoggedHandler(h http.Handler, logger *log.Logger, routeName string) http.Handler {
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           start := time.Now()
           lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
           h.ServeHTTP(lrw, r)
           logger.Printf("[%s] %s %s %s -> %d %v",
               time.Now().Format(time.RFC3339),
               r.RemoteAddr,
               r.Method,
               r.RequestURI,
               lrw.statusCode,
               time.Since(start),
           )
       })
   }
   ```
   - Grava `timestamp`, IP, método, URI, status e latência.

5. **Servidor**:
   ```go
   server := &http.Server{
       Addr:         ":80",
       Handler:      mux,
       ReadTimeout:  10 * time.Second,
       WriteTimeout: 10 * time.Second,
       IdleTimeout:  60 * time.Second,
   }
   server.ListenAndServe()
   ```

---

## 5. Criando Um Novo Serviço REST do Zero

### 5.1. Escolha de Linguagem

- **Não é necessário usar Python**. Qualquer linguagem que exponha uma rota HTTP funciona: Go, Node.js, Java, etc.
- Exemplo: FastAPI (Python), Express (Node), Gin (Go).

### 5.2. Passos Gerais para Criar um Novo Service

1. **Criar pasta no `services/`**:
   ```bash
   mkdir services/novo_service
   cd services/novo_service
   ```

2. **Implementar a lógica REST**. Exemplo com FastAPI:

   **`main.py`**:
   ```python
   from fastapi import FastAPI

   app = FastAPI()

   @app.get("/nova_rota")
   async def nova_rota():
       return {"message": "Olá do novo service!"}
   ```

   **`requirements.txt`**:
   ```
   fastapi
   hypercorn
   uvicorn
   uvloop
   ```

3. **Criar `Dockerfile`**:
   ```dockerfile
   FROM python:3.11-slim
   WORKDIR /app
   COPY requirements.txt .
   RUN pip install --no-cache-dir -r requirements.txt
   COPY main.py .
   EXPOSE 9000
   CMD ["hypercorn", "main:app", "--bind", "0.0.0.0:9000"]
   ```

4. **Atualizar `gateway/config.yml`**:
   ```yaml
   - route: /nova_rota
     target: http://novo_service:9000
     log: /var/log/gateway/nova_rota
   ```

5. **Atualizar `docker-compose.yml`**:
   ```yaml
   services:
     novo_service:
       build:
         context: ./services/novo_service
         dockerfile: Dockerfile
       image: novo_service:latest
       ports:
         - "9000:9000"
     go_gateway:
       depends_on:
         - novo_service
   ```

6. **Criar diretório de logs**:
   ```bash
   mkdir -p logs/gateway/nova_rota
   ```

7. **Testar**:
   ```bash
   docker-compose up --build
   ```
   Acesse `http://localhost:8080/nova_rota` e verifique no log `logs/gateway/nova_rota/nova_rota.log`.

---

## 6. Criando Um Novo Template Interno do Zero

### 6.1. Passos para Template

1. **Criar pasta em `templates/`**:
   ```bash
   mkdir templates/novo_template
   cd templates/novo_template
   ```

2. **Criar `index.html`**:
   ```html
   <!DOCTYPE html>
   <html lang="pt-BR">
   <head>
       <meta charset="UTF-8">
       <title>Novo Template</title>
   </head>
   <body>
       <h1>Bem-vindo ao Novo Template</h1>
       <p>Esta é uma página estática servida pelo Gateway em Go.</p>
   </body>
   </html>
   ```

3. **Atualizar `gateway/config.yml`**:
   ```yaml
   - route: /novo_template
     templateDir: /root/templates/novo_template
     log: /var/log/gateway/novo_template
     auth: public
   ```

4. **Diretórios de logs**:
   ```bash
   mkdir -p logs/gateway/novo_template
   ```

5. **Atualizar `docker-compose.yml`** (se necessário, volume já mapeado):
   ```yaml
   go_gateway:
     volumes:
       - ./templates:/root/templates
       - ./logs/gateway:/var/log/gateway
   ```

6. **Rodar**:
   ```bash
   docker-compose up --build
   ```
   Acesse `http://localhost:8080/novo_template/`.

---

## 7. Exemplo de Uso

### 7.1. Testando /health

```bash
curl http://localhost:8080/health
# Esperado: {"status":"ok"}
```

Verifique no log:
```bash
cat logs/gateway/health/health.log
# Exemplo de entrada:
# [2025-06-04T20:00:00Z] 172.17.0.1 GET /health -> 200 2.345ms
```

### 7.2. Testando /template_health

Abra navegador em:
```
http://localhost:8080/template_health/
```
Deve mostrar o HTML do `index.html`.

Verifique o log:
```bash
cat logs/gateway/template_health/template_health.log
# Exemplo de entrada:
# [2025-06-04T20:01:00Z] 172.17.0.1 GET /template_health/ -> 200 0.123ms
```

---

## 8. Dicas Finais

- **Nomes de rota**: sempre inicie com `/`.
- **templateDir**: o caminho deve ser absoluto dentro do container (ex: `/root/templates/xyz`).
- **Log**: path absoluto no container; montamos `./logs/gateway` no host em `/var/log/gateway`.
- **Proxy**: se `target` tiver múltiplos URLs separados por vírgula, o Gateway faz load balancing round‑robin com health check.

---

**Link para download do README em formato Markdown**:
[Baixar README.md](sandbox:/mnt/data/README.md)
---

## 9. Autenticação via JWT com `auth: private`

O Gateway agora permite proteger rotas com autenticação por token JWT, assinados com chave secreta segura. O controle é feito via `auth: private` no `config.yml`.

### 9.1. Como funciona

- Rotas com `auth: public` são abertas, sem exigência de autenticação.
- Rotas com `auth: private` exigem um token JWT válido no header:

```http
Authorization: Bearer <token>
```

### 9.2. Obtendo o token

Para obter um token JWT válido, faça uma requisição GET para:

```bash
curl "http://localhost:8080/auth?login=admin&password=senha123"
```

Resposta esperada:

```json
{"token":"<jwt_assinado>"}
```

Esse token pode ser usado nas rotas protegidas do Gateway.

### 9.3. Configuração necessária

Adicione ao seu `docker-compose.yml`:

```yaml
environment:
  - GATEWAY_USER=admin
  - GATEWAY_PASS=senha123
  - GATEWAY_JWT_SECRET=chaveUltraSeguraAqui
```

A chave `GATEWAY_JWT_SECRET` será usada internamente para assinar o token.

> Use uma chave forte gerada, por exemplo, com:
> `openssl rand -base64 32`

### 9.4. Segurança do JWT

- **JWT não é criptografado**, apenas **assinado**.
- O conteúdo pode ser lido, mas **não pode ser modificado** sem invalidar a assinatura.
- O segredo usado na assinatura **não é incluído no token**.
