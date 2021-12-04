package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"

	loggly "github.com/JamesPEarly/loggly"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"

	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/gorilla/mux"
)

type resTime struct {
	SystemTime string
}

type TableStatus struct {
	Table string `json:"table"`
	Count *int64 `json:"recordCount"`
}

type Item struct {
	Time     string    `json:"Time"`
	Id       string    `json:"Id"`
	Stations []Station `json:"Stations"`
}

type Station struct {
	EmptySlots int    `json:"empty_slots"`
	FreeBikes  int    `json:"free_bikes"`
	Name       string `json:"name"`
	Extra      Extra  `json:"extra"`
	Id         string `json:"id"`
}

type Extra struct {
	Renting   int `json:"renting"`
	Returning int `json:"returning"`
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func ServerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	sysTime := resTime{time.Now().String()}
	json.NewEncoder(w).Encode(sysTime)
}

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Initialize a session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		log.Fatalf("Got error initializing AWS: %s", err)
	}

	// Create DynamoDB client
	svc := dynamodb.New(sess)

	// Describe the table
	input := &dynamodb.DescribeTableInput{
		TableName: aws.String("akc-citybikes"),
	}
	
	result, err := svc.DescribeTable(input)
	if err != nil {
		log.Fatalf("Got error describing table: %s", err)
	}

	// Create response struct to be turned into JSON
	var statusResponse TableStatus
	statusResponse.Table = "akc-citybikes"
	statusResponse.Count = result.Table.ItemCount
	
	// JSON Response
	json.NewEncoder(w).Encode(statusResponse)
}

func AllHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Initialize a session
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		log.Fatalf("Got error initializing AWS: %s", err)
	}

	// Create DynamoDB client
	svc := dynamodb.New(sess)

	var allResponse []Item

	// Scan the DB for all items
	scanErr := svc.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String("akc-citybikes"),
	}, func(page *dynamodb.ScanOutput, last bool) bool {
		recs := []Item{}

		err := dynamodbattribute.UnmarshalListOfMaps(page.Items, &recs)
		if err != nil {
			panic(fmt.Sprintf("failed to unmarshal Dynamodb Scan Items, %v", err))
		}

		allResponse = append(allResponse, recs...)

		return true
	})

	// Error scanning DB
	if scanErr != nil {
		panic(fmt.Sprintf("Got error scanning DB, %v", scanErr))
	}

	// JSON Response
	json.NewEncoder(w).Encode(allResponse)
}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	searchDate := mux.Vars(r)["date"]

	proper, err := regexp.MatchString("^\\d{4}\\-(0[1-9]|1[012])\\-(0[1-9]|[12][0-9]|3[01])$", searchDate)

	if err != nil {
		log.Fatal(err)
	}

	if (proper) {
		w.WriteHeader(http.StatusOK)

		// Initialize a session
		sess, err := session.NewSession(&aws.Config{
			Region: aws.String("us-east-1")},
		)
		if err != nil {
			log.Fatalf("Got error initializing AWS: %s", err)
		}

		// Create DynamoDB client
		svc := dynamodb.New(sess)

		// Make the expression
		filt := expression.Contains(expression.Name("Time"), searchDate)

		expr, err := expression.NewBuilder().WithFilter(filt).Build()
		if err != nil {
			log.Fatalf("Got error building expression: %s", err)
		}

		// Build the query input parameters
		params := &dynamodb.ScanInput{
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			FilterExpression:          expr.Filter(),
			ProjectionExpression:      expr.Projection(),
			TableName:                 aws.String("akc-citybikes"),
		}

		// Get all data for given date
		out, err := svc.Scan(params)

		if err != nil {
			log.Fatalf("Query API call failed: %s", err)
		}

		// Unmarshal response
		searchResponse := []Item{}
		err = dynamodbattribute.UnmarshalListOfMaps(out.Items, &searchResponse)
		if err != nil {
			panic(fmt.Sprintf("Failed to unmarshal Record, %v", err))
		}

		// JSON Response
		json.NewEncoder(w).Encode(searchResponse)
	} else {
		w.WriteHeader(http.StatusBadRequest)
		badMessage := "Search should be formatted with search?date=yyyy-mm-dd"
		json.NewEncoder(w).Encode(badMessage)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lrw := NewLoggingResponseWriter(w)
		next.ServeHTTP(lrw, r)

		// Tag + client init for Loggly + send message
		var tag string = "server"
		client := loggly.New(tag)
		client.EchoSend("info", "Method type: "+r.Method+" | Source IP address: "+r.RemoteAddr+" | Request Path: "+r.RequestURI+" | Status Code: "+strconv.Itoa(lrw.statusCode))
	})
}


func main() {
	r := mux.NewRouter()
	r.HandleFunc("/akc/server", ServerHandler).Methods("GET")
	r.HandleFunc("/akc/all", AllHandler).Methods("GET")
	r.HandleFunc("/akc/status", StatusHandler).Methods("GET")
	r.HandleFunc("/akc/search", SearchHandler).Queries("date", "{date:.*}")
	wrappedRouter := loggingMiddleware(r)
	http.ListenAndServe(":8080", wrappedRouter)
}
