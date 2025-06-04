# Microservices API Gateway

Este projeto demonstra uma arquitetura de API Gateway em Go, orquestrando múltiplos microserviços em Python (FastAPI) via Docker Compose. Ele foi projetado para ser escalável, de baixa latência e facilmente extensível.

---

## Sumário

1. [Visão Geral](#visão-geral)  
2. [Motivação e Benefícios](#motivação-e-benefícios)  
3. [Como Funciona](#como-funciona)  
4. [Estrutura de Pastas](#estrutura-de-pastas)  
5. [Como Iniciar o Projeto](#como-iniciar-o-projeto)  
6. [Como Criar um Novo Serviço](#como-criar-um-novo-serviço)  
7. [Gerando `docker-compose.yml` Automaticamente](#gerando-docker-composeyml-automaticamente)  
8. [Comandos Úteis](#comandos-úteis)

---

## Visão Geral

O API Gateway, escrito em Go, atua como ponto único de entrada para diversos microserviços. Cada microserviço é desenvolvido em Python usando FastAPI e é executado em seu próprio container Docker. O Gateway roteia requisições HTTP recebidas para os serviços corretos, aproveitando HTTP/2 (h2c) para comunicação interna de alta performance.

### Principais Componentes

- **Gateway (Go)**:  
  - Carrega um arquivo de configuração YAML (`gateway/config.yml`) que define rotas e seus destinos (`target`).  
  - Cria um servidor HTTP que realiza o _reverse proxy_ de requisições para os serviços Python, usando HTTP/2 h2c internamente.  
  - Registra dois patterns (`/path` e `/path/`) para cada rota, evitando redirecionamentos indesejados.

- **Microserviços (Python + FastAPI)**:  
  - Cada microserviço vive em uma pasta em `services/<nome_do_servico>/`.  
  - Expõe seus endpoints via FastAPI, rodando sob Hypercorn/uvloop em uma porta específica.  
  - Cada serviço tem seu próprio `Dockerfile`, gerado automaticamente pelo script `generate_compose.py`, que define como empacotá-lo.

- **Docker Compose**:  
  - Orquestra os containers do Gateway e dos microserviços.  
  - Gera réplicas separadas para cada serviço via `docker-compose.yml`, definindo `build`, `ports` e dependências (`depends_on`).

---

## Motivação e Benefícios

1. **Baixa Latência**:  
   - O Go gerencia conexões com goroutines leves e usa HTTP/2 (h2c) para comunicação interna, evitando overhead de conexões repetidas.  
   - FastAPI + Hypercorn/uvloop garante alta performance no lado Python (I/O-bound).

2. **Escalabilidade**:  
   - Cada microserviço roda isolado em seu próprio container, permitindo escalar horizontalmente (aumentar réplicas).  
   - O Gateway pode ser replicado atrás de um load balancer externo se necessário.

3. **Desenvolvimento Simples**:  
   - Para adicionar um novo serviço, basta criar a pasta em `services/`, colocar `app.py` e `requirements.txt`, editar o `gateway/config.yml` e rodar `generate_compose.py`.  
   - O script se encarrega de criar `Dockerfile` e atualizar `docker-compose.yml` automaticamente.

4. **Manutenção Centralizada**:  
   - As rotas públicas ficam registradas em um único arquivo (`gateway/config.yml`).  
   - O Gateway em Go não precisa de reimplementação ao adicionar serviços; ele apenas relê o YAML.

---

## Como Funciona

1. Ao rodar `docker-compose up --build`, o script `generate_compose.py` **gera** (ou atualiza) o `docker-compose.yml` na raiz, baseado no `gateway/config.yml` e nas pastas em `services/`.  
2. Cada pasta em `services/<nome>/` recebe um `Dockerfile` genérico, configurado para rodar o FastAPI via Hypercorn na porta especificada no `target`.  
3. O Docker Compose sobe os containers:
   - `health_service` (por exemplo) escuta em `:8000`.  
   - `echo_service` (por exemplo) escuta em `:9000`.  
   - `go_gateway` escuta em `:80`, mapeado para `localhost:8080`.  

4. Quando uma requisição HTTP chega a `http://localhost:8080/<rota>`, o Gateway:
   - Verifica o `gateway/config.yml` e encontra o `target`.  
   - Encaminha a requisição internamente para `http://<service_name>:<porta>/<caminho>` via HTTP/2 h2c (h2c = HTTP/2 sem TLS).  
   - Retorna a resposta do serviço Python para o cliente.

---

## Estrutura de Pastas

```
project-root/
│   docker-compose.yml          ← gerado pelo generate_compose.py
│   generate_compose.py         ← script de geração automática
│
├───gateway
│   │   config.yml              ← define rota/target para o Gateway
│   │   Dockerfile              ← Dockerfile do Gateway em Go
│   │   go.mod                  
│   │   go.sum                  
│   │   main.go                 ← código-fonte do Gateway
│
└───services
    ├───health_service
    │       app.py              ← FastAPI code
    │       requirements.txt     ← dependências Python
    │       Dockerfile          ← gerado automaticamente
    │
    └───echo_service
            app.py              ← FastAPI code
            requirements.txt     ← dependências Python
            Dockerfile          ← gerado automaticamente
```

---

## Como Iniciar o Projeto

1. **Certifique-se de ter**:
   - Docker e Docker Compose instalados.  
   - Go 1.23 (para compilar o Gateway).  
   - Python 3.11 (para rodar `generate_compose.py`).

2. **Gere/Atualize os arquivos automaticamente**:
   ```bash
   cd project-root
   python3 generate_compose.py
   ```
   - Isso vai criar (ou sobrescrever) `docker-compose.yml` e fornecer `Dockerfile` para cada serviço em `services/`.

3. **No diretório `gateway/`, gere o `go.sum`**:
   ```bash
   cd gateway
   go mod tidy
   cd ..
   ```
   - Garante que `go.sum` exista e esteja compatível com `go.mod`.

4. **Construa e suba todos os containers**:
   ```bash
   docker-compose up --build
   ```
   - O Compose irá:
     - Buildar cada microserviço (`health_service`, `echo_service`, etc.).  
     - Buildar o Gateway (Go).  
     - Subir tudo na ordem apropriada (devido ao `depends_on`).

5. **Aguarde alguns segundos** até que todos os serviços FastAPI estejam prontos.

6. **Teste as rotas**:
   - Health Service:
     ```bash
     curl -i http://localhost:8080/health
     ```
   - Echo Service:
     ```bash
     curl -i -X POST http://localhost:8080/echo \
          -H "Content-Type: application/json" \
          -d '{"msg":"Olá, mundo!"}'
     ```

---

## Como Criar um Novo Serviço

Para adicionar um serviço chamado, por exemplo, `foo_service`:

1. **Crie a pasta** `services/foo_service/`:
   ```bash
   mkdir services/foo_service
   ```

2. **Dentro de `services/foo_service/`, crie**:
   - `app.py` com seus endpoints FastAPI (exemplo abaixo).  
   - `requirements.txt` listando dependências (`fastapi`, `hypercorn`, `uvloop`, etc.).

   Exemplo de `app.py`:
   ```python
   # services/foo_service/app.py

   from fastapi import FastAPI

   app = FastAPI()

   @app.get("/foo")
   async def foo():
       return {"message": "Resposta de foo_service!"}
   ```

   Exemplo de `requirements.txt`:
   ```
   fastapi
   uvicorn
   hypercorn
   uvloop
   ```

3. **Atualize o** `gateway/config.yml`, adicionando:
   ```yaml
   - route: /foo
     target: http://foo_service:7000
   ```
   - Defina a porta desejada (no exemplo, `7000`).

4. **Reexecute o script de geração**:
   ```bash
   python3 generate_compose.py
   ```
   - Isso criará `services/foo_service/Dockerfile` (usando o template genérico com `EXPOSE 7000`).  
   - Atualizará `docker-compose.yml`, adicionando a seção `foo_service` e incluindo-o em `gateway.depends_on`.

5. **(Opcional) Atualize `go.mod` se tiver mudado algo no Gateway**:
   ```bash
   cd gateway
   go mod tidy
   cd ..
   ```

6. **Rebuild e suba os containers**:
   ```bash
   docker-compose up --build
   ```

7. **Teste o novo endpoint**:
   ```bash
   curl -i http://localhost:8080/foo
   ```

---

## Gerando `docker-compose.yml` Automaticamente

Todo o arquivo `docker-compose.yml` é gerado pelo script `generate_compose.py`. Ele lê `gateway/config.yml` e detecta todas as pastas `services/<nome>`:

- **Para cada entrada em `gateway/config.yml`**:
  - Extrai o `service_name` e a `port` a partir de `target` (ex: `http://echo_service:9000` → `service_name="echo_service"`, `port=9000`).  
  - Gera `services/<service_name>/Dockerfile` (caso não exista), preenchendo `<PORT>`.  
  - Inclui uma seção no Compose para o serviço, expondo o mapeamento `"<port>:<port>"`.  

- A seção `gateway` em `docker-compose.yml` é montada para depender de todos esses `service_name`s, garantindo que o Gateway só tente iniciar depois que eles existam.

Isso facilita o fluxo de desenvolvimento: basta editar `gateway/config.yml` e criar a pasta do serviço; tudo o mais é automático.

---

## Comandos Úteis

- Gerar/Atualizar Dockerfiles e `docker-compose.yml`:
  ```bash
  python3 generate_compose.py
  ```

- Atualizar dependências Go (dentro de `gateway/`):
  ```bash
  cd gateway
  go mod tidy
  cd ..
  ```

- Subir todos os containers (build + run):
  ```bash
  docker-compose up --build
  ```

- Subir em modo _detached_ (background):
  ```bash
  docker-compose up --build -d
  ```

- Parar e remover containers:
  ```bash
  docker-compose down
  ```

- Testar via curl:
  ```bash
  curl -i http://localhost:8080/health
  curl -i -X POST http://localhost:8080/echo \
       -H "Content-Type: application/json" \
       -d '{"msg":"Olá!"}'
  ```

---

## Por que essa Abordagem

- **Automação**: Automatiza a geração de Dockerfiles e Compose, reduzindo erros manuais.
- **Escalabilidade**: Cada serviço roda em seu container; basta escalar réplicas no Compose ou em orquestrador.
- **Desenvolvimento Rápido**: Para adicionar um novo serviço, crie a pasta, adicione `app.py` e `requirements.txt`, ajuste `gateway/config.yml` e rode o script.  
- **Baixa Latência**: Go Gateway usa HTTP/2 (h2c) internamente e goroutines leves para alto throughput.

---

**Download deste README**: [Clique aqui para baixar o README.md](sandbox:/mnt/data/README.md)
