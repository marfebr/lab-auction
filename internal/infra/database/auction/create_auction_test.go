package auction

import (
	"context"
	"fmt"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func setupMongoContainer(ctx context.Context, t *testing.T) (*mongo.Database, func()) {
	mongodbContainer, err := mongodb.RunContainer(ctx, testcontainers.WithImage("mongo:6"))
	if err != nil {
		t.Fatal(err)
	}

	endpoint, err := mongodbContainer.Endpoint(ctx, "")
	if err != nil {
		t.Fatal(err)
	}

	mongoURI := fmt.Sprintf("mongodb://%s", endpoint)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatal(err)
	}

	database := client.Database("testdb")

	cleanup := func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Errorf("failed to disconnect from mongo: %v", err)
		}
		if err := mongodbContainer.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}

	return database, cleanup
}

func TestCreateAuction_AutoClose(t *testing.T) {
	ctx := context.Background()
	database, cleanup := setupMongoContainer(ctx, t)
	defer cleanup()

	// Configurar intervalo curto para o teste
	os.Setenv("AUCTION_INTERVAL", "2s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	// Criar repositório
	repo := NewAuctionRepository(database)

	// Criar leilão
	auction, err := auction_entity.CreateAuction(
		"Produto Teste",
		"Eletrônicos",
		"Descrição do produto teste",
		auction_entity.New,
	)
	assert.Nil(t, err)
	assert.NotNil(t, auction)

	// Inserir leilão (isso deve iniciar a goroutine de auto-close)
	internalErr := repo.CreateAuction(ctx, auction)
	assert.Nil(t, internalErr)

	// Verificar que o leilão está ativo
	var auctionMongo AuctionEntityMongo
	findErr := repo.Collection.FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&auctionMongo)
	assert.Nil(t, findErr)
	assert.Equal(t, auction_entity.Active, auctionMongo.Status)

	// Aguardar um pouco mais que o intervalo configurado
	time.Sleep(3 * time.Second)

	// Verificar que o leilão foi fechado automaticamente
	findErr = repo.Collection.FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&auctionMongo)
	assert.Nil(t, findErr)
	assert.Equal(t, auction_entity.Completed, auctionMongo.Status, "O leilão deveria estar fechado após o intervalo")
}

func TestCreateAuction_MultipleAuctions_AutoClose(t *testing.T) {
	ctx := context.Background()
	database, cleanup := setupMongoContainer(ctx, t)
	defer cleanup()

	// Configurar intervalo curto para o teste
	os.Setenv("AUCTION_INTERVAL", "2s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	// Criar repositório
	repo := NewAuctionRepository(database)

	// Criar múltiplos leilões
	auctions := []*auction_entity.Auction{}
	for i := 0; i < 5; i++ {
		auction, err := auction_entity.CreateAuction(
			fmt.Sprintf("Produto %d", i),
			"Eletrônicos",
			"Descrição do produto teste",
			auction_entity.New,
		)
		assert.Nil(t, err)
		assert.NotNil(t, auction)

		internalErr := repo.CreateAuction(ctx, auction)
		assert.Nil(t, internalErr)

		auctions = append(auctions, auction)
	}

	// Verificar que todos estão ativos
	for _, auction := range auctions {
		var auctionMongo AuctionEntityMongo
		findErr := repo.Collection.FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&auctionMongo)
		assert.Nil(t, findErr)
		assert.Equal(t, auction_entity.Active, auctionMongo.Status)
	}

	// Aguardar o fechamento
	time.Sleep(3 * time.Second)

	// Verificar que todos foram fechados
	for i, auction := range auctions {
		var auctionMongo AuctionEntityMongo
		findErr := repo.Collection.FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&auctionMongo)
		assert.Nil(t, findErr)
		assert.Equal(t, auction_entity.Completed, auctionMongo.Status, 
			fmt.Sprintf("O leilão %d deveria estar fechado", i))
	}
}

func TestCreateAuction_AlreadyCompleted_NotUpdated(t *testing.T) {
	ctx := context.Background()
	database, cleanup := setupMongoContainer(ctx, t)
	defer cleanup()

	// Configurar intervalo curto para o teste
	os.Setenv("AUCTION_INTERVAL", "2s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	// Criar repositório
	repo := NewAuctionRepository(database)

	// Criar leilão
	auction, err := auction_entity.CreateAuction(
		"Produto Teste",
		"Eletrônicos",
		"Descrição do produto teste",
		auction_entity.New,
	)
	assert.Nil(t, err)

	// Inserir leilão
	internalErr := repo.CreateAuction(ctx, auction)
	assert.Nil(t, internalErr)

	// Fechar o leilão manualmente antes que a goroutine execute
	time.Sleep(500 * time.Millisecond)
	_, updateErr := repo.Collection.UpdateOne(
		ctx,
		bson.M{"_id": auction.Id},
		bson.M{"$set": bson.M{"status": auction_entity.Completed}},
	)
	assert.Nil(t, updateErr)

	// Aguardar o tempo da goroutine
	time.Sleep(2 * time.Second)

	// Verificar que o status continua Completed
	var auctionMongo AuctionEntityMongo
	findErr := repo.Collection.FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&auctionMongo)
	assert.Nil(t, findErr)
	assert.Equal(t, auction_entity.Completed, auctionMongo.Status)
}

func TestGetAuctionInterval_DefaultValue(t *testing.T) {
	// Remover variável de ambiente se existir
	os.Unsetenv("AUCTION_INTERVAL")

	interval := getAuctionInterval()
	assert.Equal(t, 5*time.Minute, interval, "Deve retornar 5 minutos como fallback")
}

func TestGetAuctionInterval_CustomValue(t *testing.T) {
	// Definir valor customizado
	os.Setenv("AUCTION_INTERVAL", "10m")
	defer os.Unsetenv("AUCTION_INTERVAL")

	interval := getAuctionInterval()
	assert.Equal(t, 10*time.Minute, interval)
}

func TestGetAuctionInterval_InvalidValue(t *testing.T) {
	// Definir valor inválido
	os.Setenv("AUCTION_INTERVAL", "invalid")
	defer os.Unsetenv("AUCTION_INTERVAL")

	interval := getAuctionInterval()
	assert.Equal(t, 5*time.Minute, interval, "Deve retornar 5 minutos quando o valor é inválido")
}

func TestCreateAuction_WithDifferentIntervals(t *testing.T) {
	ctx := context.Background()
	database, cleanup := setupMongoContainer(ctx, t)
	defer cleanup()

	tests := []struct {
		name          string
		interval      string
		waitTime      time.Duration
		expectClosed  bool
	}{
		{
			name:         "1 segundo - deve fechar",
			interval:     "1s",
			waitTime:     2 * time.Second,
			expectClosed: true,
		},
		{
			name:         "5 segundos - não deve fechar ainda",
			interval:     "5s",
			waitTime:     2 * time.Second,
			expectClosed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("AUCTION_INTERVAL", tt.interval)
			defer os.Unsetenv("AUCTION_INTERVAL")

			repo := NewAuctionRepository(database)

			auction, err := auction_entity.CreateAuction(
				"Produto Teste",
				"Eletrônicos",
				"Descrição do produto teste",
				auction_entity.New,
			)
			assert.Nil(t, err)

			internalErr := repo.CreateAuction(ctx, auction)
			assert.Nil(t, internalErr)

			time.Sleep(tt.waitTime)

			var auctionMongo AuctionEntityMongo
			findErr := repo.Collection.FindOne(ctx, bson.M{"_id": auction.Id}).Decode(&auctionMongo)
			assert.Nil(t, findErr)

			if tt.expectClosed {
				assert.Equal(t, auction_entity.Completed, auctionMongo.Status,
					"O leilão deveria estar fechado")
			} else {
				assert.Equal(t, auction_entity.Active, auctionMongo.Status,
					"O leilão ainda deveria estar ativo")
			}
		})
	}
}
