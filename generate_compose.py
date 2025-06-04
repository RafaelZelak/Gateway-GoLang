#!/usr/bin/env python3
# generate_compose.py
#
# This script auto-generates:
#  1) A Dockerfile for each folder under services/ (if not present),
#     replacing <PORT> in the template based on config.yml.
#  2) A docker-compose.yml in the project root, containing
#     the gateway service and all microservices under services/.
#
# Usage:
#   cd project-root
#   python3 generate_compose.py

import os
import yaml
import textwrap

# ─── Paths e constantes ─────────────────────────────────────────────────────────
ROOT_DIR = os.path.abspath(os.path.dirname(__file__))
SERVICES_DIR = os.path.join(ROOT_DIR, "services")
GATEWAY_DIR = os.path.join(ROOT_DIR, "gateway")
CONFIG_PATH = os.path.join(GATEWAY_DIR, "config.yml")
OUTPUT_COMPOSE = os.path.join(ROOT_DIR, "docker-compose.yml")

# Template Dockerfile para cada microservice. <PORT> será substituído.
DOCKERFILE_TEMPLATE = textwrap.dedent("""
    # Dockerfile template for any FastAPI-based service

    # Stage 1: install Python dependencies
    FROM python:3.11-slim AS builder

    WORKDIR /app

    COPY requirements.txt .
    RUN pip install --no-cache-dir -r requirements.txt

    COPY app.py .

    # Stage 2: runtime image
    FROM python:3.11-slim

    WORKDIR /app

    # Copy installed packages from builder
    COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
    COPY --from=builder /usr/local/bin /usr/local/bin

    # Copy application code
    COPY --from=builder /app/app.py .

    # Expose port (service runs on <PORT>)
    EXPOSE {port}

    # Start the service via Hypercorn (listens on 0.0.0.0:{port})
    CMD ["hypercorn", "app:app", "--bind", "0.0.0.0:{port}", "--workers", "4", "--worker-class", "uvloop"]
""")


def load_gateway_config(path):
    """
    Load the gateway/config.yml file and return a list of dicts:
       [{'route': '/health', 'target': 'http://health_service:8000'}, ...]
    """
    with open(path, "r", encoding="utf-8") as f:
        data = yaml.safe_load(f)
    return data.get("services", [])


def ensure_service_dockerfile(service_name, port):
    """
    If services/<service_name>/Dockerfile does not exist, create it using the template.
    """
    service_dir = os.path.join(SERVICES_DIR, service_name)
    dockerfile_path = os.path.join(service_dir, "Dockerfile")
    if not os.path.isdir(service_dir):
        print(f"Warning: service directory '{service_name}' does not exist under 'services/'. Skipping.")
        return

    if os.path.exists(dockerfile_path):
        print(f"Dockerfile already exists for service '{service_name}', skipping creation.")
        return

    content = DOCKERFILE_TEMPLATE.format(port=port)
    with open(dockerfile_path, "w", encoding="utf-8") as f:
        f.write(content.strip() + "\n")
    print(f"Created Dockerfile for service '{service_name}' with port {port}.")


def generate_docker_compose(services):
    """
    Generate a docker-compose.yml with:
      - gateway service
      - one entry per service in 'services' list
    """
    compose = {
        "version": "3.8",
        "services": {}
    }

    # 1) Gateway entry
    compose["services"]["gateway"] = {
        "build": {
            "context": "./gateway",
            "dockerfile": "Dockerfile"
        },
        "container_name": "go_gateway",
        "ports": ["8080:80"],
        "depends_on": []
    }

    # 2) For each microservice
    for svc in services:
        target = svc["target"]  # e.g. "http://health_service:8000"
        # extrair service_name e porta do target
        # target sempre no formato: http://<service_name>:<port>
        prefix = "http://"
        if not target.startswith(prefix):
            print(f"Warning: unexpected target format '{target}'. Skipping.")
            continue

        no_proto = target[len(prefix):]        # e.g. "health_service:8000"
        if ":" not in no_proto:
            print(f"Warning: no port found in target '{target}'. Skipping.")
            continue

        service_name, port_str = no_proto.split(":", 1)
        try:
            port = int(port_str)
        except ValueError:
            print(f"Warning: invalid port '{port_str}' in target '{target}'. Skipping.")
            continue

        # Gera Dockerfile em services/<service_name> se não existir
        ensure_service_dockerfile(service_name, port)

        # Adiciona entry no docker-compose
        compose["services"][service_name] = {
            "build": {
                "context": f"./services/{service_name}",
                "dockerfile": "Dockerfile"
            },
            "container_name": service_name,
            "ports": [f"{port}:{port}"]
        }

        # Adiciona este service_name em depends_on do gateway
        compose["services"]["gateway"]["depends_on"].append(service_name)

    # 3) Escreve o docker-compose.yml
    with open(OUTPUT_COMPOSE, "w", encoding="utf-8") as f:
        yaml.dump(compose, f, sort_keys=False)
    print(f"Generated '{OUTPUT_COMPOSE}' with services: {list(compose['services'].keys())}")


if __name__ == "__main__":
    # 1) Ler config.yml do gateway
    services_config = load_gateway_config(CONFIG_PATH)
    # 2) Gerar Dockerfiles e docker-compose.yml
    generate_docker_compose(services_config)
