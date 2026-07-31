package main

import (
	"context"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/lealre/movies-backend/internal/mongodb"
	"go.mongodb.org/mongo-driver/bson"
)

func main() {
	_ = godotenv.Load()

	ctx := context.Background()
	dbClient, err := mongodb.Connect(ctx)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer dbClient.Disconnect(ctx)

	db := mongodb.NewDB(dbClient)
	database := dbClient.Database(db.GetDatabaseName())

	fmt.Println("🔄 Backfilling deleted=false on groups missing the field...")
	coll := database.Collection(mongodb.GroupsCollection)
	res, err := coll.UpdateMany(
		ctx,
		bson.M{"deleted": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"deleted": false}},
	)
	if err != nil {
		log.Fatalf("Failed to backfill: %v", err)
	}
	fmt.Printf("✅ Backfilled %d groups\n", res.ModifiedCount)
	fmt.Println("⚠️  Now rebuild the group unique index (reset) so the new partial filter applies.")
}
