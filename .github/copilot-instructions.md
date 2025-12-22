# Sistema de Leilões - Instruções para Agentes de IA

## Visão Geral do Projeto
Sistema de leilões baseado em Go com fechamento automático baseado em tempo. Utiliza padrão Clean Architecture com backend MongoDB.

**Tarefa Atual**: Implementar fechamento automático de leilões usando goroutines quando `AUCTION_INTERVAL` expirar.

## Padrão de Arquitetura

### Fluxo de Camadas (Entrada)
```
Requisição HTTP → Controller → UseCase → Repository → MongoDB
```

### Diretórios Principais
- `internal/entity/*_entity/` - Entidades de domínio com validação de negócio
- `internal/usecase/*_usecase/` - Orquestração da lógica de negócio
- `internal/infra/api/web/controller/` - Handlers HTTP (framework Gin)
- `internal/infra/database/` - Repositórios MongoDB
- `configuration/` - Configurações compartilhadas (logger, tratamento de erros, conexão DB)

### Injeção de Dependências
Veja `cmd/auction/main.go:initDependencies()` - DI manual que cria a cadeia Repository → UseCase → Controller. Nenhum framework de DI é usado.

## Padrões Críticos

### Tratamento de Erros (Sistema de Duas Camadas)
```go
// Camada interna (domain/usecase/repository)
return internal_error.NewBadRequestError("message")  // tipos: BadRequest, NotFound, InternalServer

// Camada REST (controllers)
restErr := rest_err.ConvertError(internalErr)  // converte para códigos de status HTTP
c.JSON(restErr.Code, restErr)
```

Nunca retorne erros diretamente das camadas internas para HTTP - sempre use a conversão de duas camadas.

### Padrão de Criação de Entidades
Entidades usam funções factory com validação:
```go
auction, err := auction_entity.CreateAuction(name, category, desc, condition)
if err != nil { return err }  // validação acontece na factory
```

### Mapeamento MongoDB
Cada repositório define uma struct paralela `*EntityMongo` com tags BSON:
- `Timestamp` armazenado como Unix int64
- `_id` é string UUID, não ObjectID do MongoDB
- Converte de volta para entidade de domínio nas consultas do repositório

### Padrão de Concorrência (Referência: `bid/create_bid.go`)
```go
// Cache de status com mutexes
auctionStatusMap      map[string]auction_entity.AuctionStatus
auctionStatusMapMutex *sync.Mutex

// Cálculo de tempo a partir de variável de ambiente
auctionInterval := getAuctionInterval()  // lê a variável AUCTION_INTERVAL
auctionEndTime := auctionEntity.Timestamp.Add(auctionInterval)

// Padrão de goroutine
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func(val Item) {
        defer wg.Done()
        // trava antes de acessar o map
        mutex.Lock()
        status := statusMap[key]
        mutex.Unlock()
        // ... trabalho ...
    }(item)
}
wg.Wait()
```

**Importante**: O repositório de Bid mostra como verificar expiração do leilão. O repositório de Auction precisa de goroutine similar para fechar automaticamente leilões expirados.

## Configuração de Ambiente
Localizada em `cmd/auction/.env`:
```
AUCTION_INTERVAL=10m          # Formato de duração: 1h, 30m, 90s
MONGODB_URL=mongodb://...
MONGODB_DB=auctions
```

Use `time.ParseDuration()` para parsear `AUCTION_INTERVAL` (fallback para 5 minutos).

## Fluxo de Desenvolvimento

### Executar Localmente
```bash
docker-compose up --build  # Inicia app na :8080 e MongoDB na :27017
```

### Testes Manuais
```bash
# Criar leilão
curl -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{"product_name":"Item","category":"Electronics","description":"Description here","condition":0}'

# Listar leilões
curl http://localhost:8080/auction

# Criar lance
curl -X POST http://localhost:8080/bid \
  -H "Content-Type: application/json" \
  -d '{"user_id":"<uuid>","auction_id":"<uuid>","amount":100.50}'
```

### Constantes de Status do Leilão
```go
const (
    Active AuctionStatus = iota  // 0
    Completed                    // 1
)
```

## Foco da Implementação: Funcionalidade de Fechamento Automático

### O que Já Existe
- ✅ Validação de bid verifica `auction.Timestamp + AUCTION_INTERVAL` para rejeitar lances em leilões expirados
- ✅ Maps de rastreamento de status no repositório de bid

### O que Está Faltando (Sua Tarefa)
Em `internal/infra/database/auction/create_auction.go`:
1. Adicionar goroutine que verifica periodicamente leilões expirados
2. Atualizar status do leilão de `Active` (0) para `Completed` (1) quando o tempo expirar
3. Usar operação de update do MongoDB para mudar o status
4. Lidar com concorrência de forma segura (referenciar padrão de mutex no repositório de bid)

### Estratégia de Testes
- Testar que leilão fecha automaticamente após `AUCTION_INTERVAL`
- Testar que lances são rejeitados após o fechamento
- Testar que criação concorrente de leilões não causa race conditions

## Convenções Específicas do Projeto
- Ainda não existem testes - você está criando os primeiros
- Logger de `configuration/logger` para todos os logs de erro/info: `logger.Error("msg", err)`
- Validação JSON via tags de binding do Gin: `binding:"required,min=1"`
- Context passado por todas as camadas, tipicamente `context.Background()` dos controllers
- Formato UUID: `github.com/google/uuid` para todos os IDs

## Pegadinhas Comuns
- Não use `ObjectID` - todos os IDs são strings UUID
- Não esqueça `wg.Wait()` ao spawnar goroutines
- Sempre trave mutexes antes de acessar maps compartilhados
- Retorne `*internal_error.InternalError` de repositories/usecases, não erros padrão
- Conversões de timestamp: `.Unix()` para armazenar, `time.Unix(ts, 0)` para carregar
