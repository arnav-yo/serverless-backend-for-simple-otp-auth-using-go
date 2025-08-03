package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"fmt"
	"io"

	"math/rand"
	"net/smtp"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type OTP struct {
	Email     string    `json:"email" bson:"email"`
	Code      string    `json:"code" bson:"code"`
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
}

type GenerateOTPRequest struct {
	Email string `json:"email"`
}

type VerifyOTPRequest struct {
	Email string `json:"email"`
	OTP   string `json:"otp"`
}

type Response struct {
	Message string `json:"message"`
}

func Initialize() {
	err:=godotenv.Load()
	fmt.Println(err)
}


func JSONwriter (w http.ResponseWriter, statusCode int, data Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err:= json.NewEncoder(w).Encode(data); err!=nil {
		log.Printf("Error encoding the data into JSON %v",err)
		http.Error(w,"There was an internal server error.",http.StatusInternalServerError)

	}
}

func SendASimpleMail (sendTo string, OTP any, generateOTP bool) error {
	var MAILING_PASSWORD string = os.Getenv("PASSWORD_FOR_MAILING")
	auth:=smtp.PlainAuth(
		"",
		"arnavraheja10@gmail.com",
		MAILING_PASSWORD,
		"smtp.gmail.com",
	)
	if generateOTP {
		var msg string = fmt.Sprintf("Subject:Your OTP for the Go Backend Project\nThis is your OTP, It expires in exactly 2 minutes. Dont share this with anyone \n%v",OTP)

		err := smtp.SendMail(
		"smtp.gmail.com:587",
		auth,
		"arnavraheja10@gmail.com",
		[]string{sendTo},
		[]byte(msg),
	)	
	 if err!= nil {
		return err
	}

	} else {
		var msg string = fmt.Sprintf("Subject: Your email has been verified successfully!\nYour email %v has been verified successfully via the use of an OTP.",sendTo)
		err := smtp.SendMail(
			"smtp.gmail.com:587",
			auth,
			"arnavraheja10@gmail.com",
			[]string{sendTo},
			[]byte(msg),
		)

	 	if err != nil {
			return err
		}
	}

	return nil
	
}


func connectToDB() (*mongo.Client, *mongo.Database, error) {

	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		return nil, nil, fmt.Errorf("MONGODB_URI environment variable is not set")
	}

	clientOptions := options.Client().ApplyURI(mongoURI)

	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	} else {
		fmt.Println("Connected to MongoDB successfully")
	}

	db := client.Database("Email-OTP-Database") 
	return client, db, nil
}


func HandleGenerateOTPRequest(w http.ResponseWriter, r *http.Request){
	if r.Method != "POST" {
		JSONwriter(w,http.StatusMethodNotAllowed, Response{Message:"Method NOT Allowed, use POST method instead."})
		return
	}

	body,err := io.ReadAll(r.Body)
	if err!=nil {
		JSONwriter(w,http.StatusBadRequest,Response{Message: "Could not read the body of the request that has been sent to the backend."})
		return 
	}
	var req GenerateOTPRequest
	if err := json.Unmarshal(body,&req); err!=nil{
		JSONwriter(w,http.StatusBadRequest,Response{Message:"Invalid JSON format"})
		return
	}

	if req.Email == "" {
		JSONwriter(w,http.StatusBadRequest, Response{Message: "Email is required to generate an OTP!"})
		return
	}


	client,db,err := connectToDB()

	if err!=nil {
		JSONwriter(w,http.StatusInternalServerError,Response{Message:"Could not connect to the database"})
		log.Println(err)
		return 
	}

	defer client.Disconnect(context.Background())

	var newOTP string = generateOTP()

	var otpDoc OTP

	otpDoc.Email = req.Email
	otpDoc.CreatedAt = time.Now()
	otpDoc.Code = newOTP

	otpCollection := db.Collection("OTPs")

	filter:= bson.M{"email" : otpDoc.Email}

	update:=bson.M{"$set" : otpDoc}
	
	optionsChoice:=options.Update().SetUpsert(true)

	UpdatedResultDoc,err := otpCollection.UpdateOne(context.Background(), filter , update, optionsChoice)
	_ = UpdatedResultDoc
	if err!=nil {
		log.Printf("Error updating OTP in MongoDB %v", err)
		JSONwriter(w,http.StatusInternalServerError,Response{Message:"Error updating/inserting an OTP in the DB"})
		return 
	}
	errorSendingMail := SendASimpleMail(otpDoc.Email,otpDoc.Code,true)
	if errorSendingMail!=nil {
		
		JSONwriter(w,http.StatusInternalServerError,Response{Message: "There was an error sending the OTP to your specified email."})
		fmt.Println(errorSendingMail)
		return 
	}

	JSONwriter(w,http.StatusAccepted,Response{Message:"OTP has been generated successfully and has been sent to your email id"})
	 
}

func generateOTP () string {
	resultingArray := make([]string,0,4)
	
	for i :=0 ; i<4;i++{
		_=i
		var randIndex = rand.Intn(2)
		var randomChar string = string(65+rand.Intn(26))
		var randomNumbInString string = fmt.Sprintf("%d",rand.Intn(10))
		array := [2]string{randomChar,randomNumbInString}
		randomElement := array[randIndex]
		resultingArray = append(resultingArray,randomElement)
	}

	return strings.Join(resultingArray,"")

	}

func HandleVerifyOTP(w http.ResponseWriter, r *http.Request){

	if r.Method != "POST" {
		JSONwriter(w,http.StatusMethodNotAllowed,Response{Message:"Wrong method type. Use POST method instead"})
		return 
	}

	body,err := io.ReadAll(r.Body)
	if err != nil { 
	JSONwriter(w,http.StatusBadRequest,Response{Message: "Could not read the body of the request"})
	return 
	}

	var req VerifyOTPRequest 
	
	if err := json.Unmarshal(body, &req); err != nil { 
	JSONwriter(w,http.StatusBadRequest, Response{Message: "Invalid JSON Format."})
	return 
	} 

	if req.Email == "" || req.OTP == "" { 
	JSONwriter(w, http.StatusBadRequest,Response{Message: "Email and OTP are required"} ) 
	return 
	}
	client, db, err := connectToDB() 
	if err != nil { 
	JSONwriter(w, http.StatusInternalServerError,Response{Message: "Could not connect to the database"}) 
	log.Println(err) 
	return 
	} 
	defer client.Disconnect(context.Background()) 
	otpsCollection := db.Collection("OTPs") 
	var result OTP 
	filter := bson.M{"email": req.Email, "code": req.OTP} 
	err = otpsCollection.FindOne(context.Background(), filter).Decode(&result) 
	if err != nil { 
		if err == mongo.ErrNoDocuments { 
			JSONwriter(w, http.StatusUnauthorized,Response{Message: "Invalid OTP"})
			log.Println(err,result) 
			} else { 
			JSONwriter(w,http.StatusInternalServerError, Response{Message: "Database error"}) 
			log.Println(err) 
			} 
			return 
		}

		expirationTime := result.CreatedAt.Add(2 * time.Minute) 
		if time.Now().After(expirationTime) { 
		JSONwriter(w, http.StatusUnauthorized,Response{Message:"OTP has expired. Please request a new one."}) 
		 _, _ = otpsCollection.DeleteOne(context.Background(), filter) 
		 return 
		}
		_, err = otpsCollection.DeleteOne(context.Background(), filter) 
		if err != nil {
		 JSONwriter(w,http.StatusInternalServerError,Response{Message: "Failed to delete OTP"}) 
		 log.Println(err)
		  return 
		  } 
		JSONwriter(w,http.StatusOK, Response{Message: "OTP Verified Successfuly!"})
		errorSendingMail := SendASimpleMail(result.Email,nil,false)
	if errorSendingMail!=nil {
		
		JSONwriter(w,http.StatusInternalServerError,Response{Message: "There was an error sending the OTP to your specified email."})
		fmt.Println(errorSendingMail)
		return 
	}
		
}




func main(){
	Initialize()
	http.HandleFunc("/generate-otp",HandleGenerateOTPRequest)

	http.HandleFunc("/verify-otp",HandleVerifyOTP)

	fmt.Println("Listening on Port 8080")

	

	fmt.Print(http.ListenAndServe(":8080",nil))
	
}