package db

import (
	"context"
	"fmt"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ConnectDB kết nối đến cơ sở dữ liệu MongoDB
func ConnectDB() (*mongo.Client, error) {
	// Thông tin kết nối
	host := "192.168.0.104"
	port := 7017
	user := "dangnh"
	pass := "inhdi"

	// Tạo chuỗi kết nối
	uri := fmt.Sprintf("mongodb://%s:%s@%s:%d/?tls=false", user, pass, host, port)

	// Tạo một context để sử dụng trong các hoạt động cơ sở dữ liệu
	ctx := context.Background()

	// Tạo các tùy chọn kết nối
	clientOptions := options.Client().ApplyURI(uri)

	// Tạo kết nối đến cơ sở dữ liệu MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB:", err)
		return nil, err
	}

	// Đảm bảo rằng chúng ta có thể kết nối đến MongoDB
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
		log.Fatal("Failed to ping MongoDB:", err)
		return nil, err
	}
	// Trả về con trỏ đến client và nil error
	return client, nil
}

// CloseDB đóng kết nối đến cơ sở dữ liệu MongoDB
func CloseDB(client *mongo.Client) {
	err := client.Disconnect(context.Background())
	if err != nil {
		log.Fatal("Failed to disconnect from MongoDB:", err)
	}
}



// Connect to collection
func ConnectCollection(client *mongo.Client, dbName string, collection string) (*mongo.Collection, error) {
	// Connect to the collection
	coll := client.Database(dbName).Collection(collection)
	if coll == nil {
		return nil, log.Output(1, "Failed to connect to collection")
	}
	return coll, nil
}

// Check if a collection exists
func CollectionExists(client *mongo.Client, dbName string, collection string) bool {
	// Get the list of collections in the database
	collections, err := client.Database(dbName).ListCollectionNames(context.Background(), nil)
	if err != nil {
		log.Fatal("Failed to get list of collections:", err)
	}

	// Check if the collection exists in the list of collections
	for _, c := range collections {
		if c == collection {
			return true
		}
	}
	fmt.Println("Collection does not exist with name:", collection)
	return false
}

func CreateDatabase(client *mongo.Client, dbName string, collection string) error {

	// Check if the database exists
	// get all databases
	filter := bson.M{}
	databases, err := client.ListDatabaseNames(context.Background(), filter)
	if err != nil {
		log.Fatal("Failed to get list of databases:", err)
	}
	// Check if the database exists in the list of databases
	for _, db := range databases {
		if db == dbName {
			log.Println("Database already exists:", dbName)
			return nil
		}
	}

	// Create a new database
	err = client.Database(dbName).CreateCollection(context.Background(), collection)
	if err != nil {
		log.Fatal("Failed to create database:", err)
		return err
	}
	log.Println("Created database:", dbName)
	return nil

}
// Insert a document into a collection
func InsertDocument(client *mongo.Client, dbName string, collection string, doc interface{}) error {
	// Connect to the collection
	coll, err := ConnectCollection(client, dbName, collection)
	if err != nil {
		return err
	}

	// Insert the document into the collection
	_, err = coll.InsertOne(context.Background(), doc)
	if err != nil {
		fmt.Println("Failed to insert document into collection:", err)
		return err
	}

	// Return nil error
	return nil
}

// Insert multiple documents into a collection
func InsertDocuments(client *mongo.Client, dbName string, collection string, docs []interface{}) error {
	// Connect to the collection
	coll, err := ConnectCollection(client, dbName, collection)
	if err != nil {
		return err
	}

	// Insert the documents into the collection
	_, err = coll.InsertMany(context.Background(), docs)
	if err != nil {
		fmt.Println("Failed to insert documents into collection:", err)
		return err
	}

	// Return nil error
	return nil
}