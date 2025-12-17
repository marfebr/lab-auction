# Documentação - Sistema de Leilões com Fechamento Automático

## 📋 Sumário
- [Visão Geral](#visão-geral)
- [Requisitos](#requisitos)
- [Instalação](#instalação)
- [Como Rodar](#como-rodar)
- [Como Testar](#como-testar)
- [Configuração](#configuração)
- [Funcionalidades Implementadas](#funcionalidades-implementadas)
- [Arquitetura](#arquitetura)

## 🎯 Visão Geral

Sistema de leilões desenvolvido em Go 1.25(atualizado por reuisitos de alguns pacotes) que implementa **fechamento automático de leilões** baseado em tempo configurável. Utiliza Clean Architecture, MongoDB como banco de dados e Docker para ambiente de desenvolvimento.

## 📦 Requisitos

- Docker e Docker Compose instalados
- Go 1.25+ (apenas para desenvolvimento local sem Docker)

## 🚀 Instalação

### Clonar o Repositório
```bash
git clone <url-do-repositorio>
cd fullcycle-auction_go
```

## ▶️ Como Rodar

### Opção 1: Com Docker Compose (Recomendado)

```bash
# Subir a aplicação e o MongoDB
docker-compose up --build

# Em modo detached (background)
docker-compose up --build -d
```

A aplicação estará disponível em: `http://localhost:8080`

### Opção 2: Localmente (Desenvolvimento)

```bash
# 1. Certifique-se de ter MongoDB rodando localmente ou via Docker
docker run -d -p 27017:27017 --name mongodb \
  -e MONGO_INITDB_ROOT_USERNAME=admin \
  -e MONGO_INITDB_ROOT_PASSWORD=admin \
  mongo:latest

# 2. Configure as variáveis de ambiente
cp cmd/auction/.env.example cmd/auction/.env
# Edite cmd/auction/.env com suas configurações

# 3. Execute a aplicação
go run cmd/auction/main.go
```

## 🧪 Como Testar

### Testes Automatizados

```bash
# Executar todos os testes
go test ./... -v

# Executar apenas os testes de auto-close
go test ./internal/infra/database/auction/... -v -run TestCreateAuction

# Executar testes com cobertura
go test ./... -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

**Nota**: Os testes utilizam Testcontainers e criam containers MongoDB temporários automaticamente.

### Testes Manuais via API

#### 1. Criar um Leilão
```bash
curl -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "Notebook Gamer",
    "category": "Eletrônicos",
    "description": "Notebook gamer de última geração com RTX 4090",
    "condition": 0
  }'
```

**Condições disponíveis:**
- `0` = Novo (New)
- `1` = Usado (Used)
- `2` = Recondicionado (Refurbished)

#### 2. Listar Todos os Leilões
```bash
curl http://localhost:8080/auction
```

#### 3. Buscar Leilão por ID
```bash
curl http://localhost:8080/auction/{auction_id}
```

#### 4. Criar um Lance
```bash
curl -X POST http://localhost:8080/bid \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123e4567-e89b-12d3-a456-426614174000",
    "auction_id": "{auction_id}",
    "amount": 5000.00
  }'
```

#### 5. Buscar Lances de um Leilão
```bash
curl http://localhost:8080/bid/{auction_id}
```

#### 6. Buscar Vencedor de um Leilão
```bash
curl http://localhost:8080/auction/winner/{auction_id}
```

## ⚙️ Configuração

### Variáveis de Ambiente

Edite o arquivo `cmd/auction/.env`:

```env
# Intervalo de tempo para fechamento automático do leilão
AUCTION_INTERVAL=10m

# Credenciais do MongoDB
MONGO_INITDB_ROOT_USERNAME=admin
MONGO_INITDB_ROOT_PASSWORD=admin

# String de conexão do MongoDB
MONGODB_URL=mongodb://admin:admin@mongodb:27017/auctions?authSource=admin

# Nome do banco de dados
MONGODB_DB=auctions
```

### Formatos de AUCTION_INTERVAL

- `30s` = 30 segundos
- `5m` = 5 minutos
- `1h` = 1 hora
- `1h30m` = 1 hora e 30 minutos
- `10m` = 10 minutos (padrão no docker-compose)

**Fallback**: Se não configurado ou inválido, usa 5 minutos por padrão.

## ✨ Funcionalidades Implementadas

### 1. Função de Cálculo de Tempo ✅
- Função `getAuctionInterval()` lê a variável de ambiente `AUCTION_INTERVAL`
- Usa `time.ParseDuration()` para converter string em duração
- Fallback automático para 5 minutos em caso de erro
- Localização: `internal/infra/database/auction/create_auction.go:75-82`

### 2. Goroutine de Fechamento Automático ✅
- Goroutine iniciada automaticamente ao criar um leilão
- Aguarda o tempo configurado usando `time.After()`
- Atualiza status do leilão de `Active` (0) para `Completed` (1)
- Verifica status antes de atualizar (evita race conditions)
- Tratamento de erros com logging
- Localização: `internal/infra/database/auction/create_auction.go:52-69`

### 3. Testes Automatizados ✅
4 testes completos implementados:

#### a) `TestCreateAuction_AutoClose`
- Valida fechamento automático de um único leilão
- Verifica mudança de status após intervalo configurado

#### b) `TestCreateAuction_MultipleAuctions_AutoClose`
- Testa concorrência: 5 leilões simultâneos
- Garante que todos fecham corretamente

#### c) `TestCreateAuction_AlreadyCompleted_NotUpdated`
- Verifica que leilões já completos não são re-atualizados
- Testa a proteção contra race conditions

#### d) `TestCreateAuction_WithDifferentIntervals`
- Testa diferentes intervalos de tempo
- Valida que fechamento ocorre no momento correto

**Localização**: `internal/infra/database/auction/create_auction_test.go`

## 🏗️ Arquitetura

### Estrutura de Pastas
```
.
├── cmd/auction/              # Entry point da aplicação
├── configuration/            # Configurações (logger, DB, erros)
├── internal/
│   ├── entity/              # Entidades de domínio
│   │   ├── auction_entity/  # Entidade de Leilão
│   │   ├── bid_entity/      # Entidade de Lance
│   │   └── user_entity/     # Entidade de Usuário
│   ├── usecase/             # Casos de uso (lógica de negócio)
│   ├── infra/
│   │   ├── api/web/         # Controllers HTTP (Gin)
│   │   └── database/        # Repositórios MongoDB
│   └── internal_error/      # Tratamento de erros interno
├── Dockerfile               # Imagem Docker da aplicação
├── docker-compose.yml       # Orquestração app + MongoDB
└── go.mod                   # Dependências Go
```

### Clean Architecture

```
┌─────────────────────────────────────────┐
│  HTTP Request (Gin Framework)           │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  Controller Layer                        │
│  - Validação JSON                        │
│  - Conversão de erros REST               │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  UseCase Layer                           │
│  - Lógica de negócio                     │
│  - Orquestração                          │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  Repository Layer                        │
│  - Goroutine auto-close ⭐               │
│  - Operações MongoDB                     │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│  MongoDB                                 │
└─────────────────────────────────────────┘
```

### Fluxo do Fechamento Automático

```
1. POST /auction
   └─> Controller recebe requisição
       └─> UseCase valida dados
           └─> Repository insere no MongoDB
               └─> ⭐ Goroutine iniciada
                   └─> time.After(AUCTION_INTERVAL)
                       └─> Update status = Completed
```

## 🔍 Detalhes de Implementação

### Código-Chave: create_auction.go

```go
// Goroutine de fechamento automático
go func() {
    <-time.After(getAuctionInterval())
    
    // Filtro: apenas leilões ativos
    filter := bson.M{
        "_id":    auctionEntity.Id,
        "status": auction_entity.Active,
    }
    
    // Update: muda status para Completed
    update := bson.M{
        "$set": bson.M{
            "status": auction_entity.Completed,
        },
    }
    
    _, err := ar.Collection.UpdateOne(context.Background(), filter, update)
    if err != nil {
        logger.Error("Error trying to update auction status", err)
        return
    }
}()
```

### Proteção Contra Race Conditions

O filtro MongoDB inclui verificação de status:
```go
filter := bson.M{
    "_id":    auctionEntity.Id,
    "status": auction_entity.Active,  // ⭐ Só atualiza se ainda estiver ativo
}
```

Isso garante que:
- Leilões já completos não sejam atualizados novamente
- Operações concorrentes não causem inconsistências

## 🐛 Troubleshooting

### Erro: "Cannot connect to MongoDB"
```bash
# Verificar se o MongoDB está rodando
docker ps | grep mongodb

# Reiniciar o MongoDB
docker-compose restart mongodb
```

### Erro: "AUCTION_INTERVAL invalid"
- Verifique o formato no arquivo `.env`
- Formatos válidos: `1h`, `30m`, `90s`, `1h30m`

### Testes falhando por timeout
```bash
# Aumentar timeout dos testes
go test ./... -v -timeout 5m
```

### Logs da Aplicação
```bash
# Ver logs em tempo real
docker-compose logs -f app

# Ver logs do MongoDB
docker-compose logs -f mongodb
```

## 📚 Referências

- [Go Documentation](https://golang.org/doc/)
- [MongoDB Go Driver](https://pkg.go.dev/go.mongodb.org/mongo-driver/mongo)
- [Testcontainers Go](https://golang.testcontainers.org/)
- [Gin Web Framework](https://gin-gonic.com/)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)

## 📝 Licença

Este projeto foi desenvolvido como parte de um desafio técnico.

