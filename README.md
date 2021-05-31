# Monopoly Backend

Stack (Go):
* Fiber
* Socket.io

### How to use

Make sure you have Go installed
First clone the repo ```git clone https://github.com/DedS3t/monopoly-backend && cd monopoly-backend```
Then run ```go get``` to download dependencies

The next step is to setup postresql...
1. Create a new database
2. Run the db.sql to setup the database
3. Create a .env file with the following keys
    * DB_HOST
    * DB_PORT
    * DB_USER
    * DB_PASSWORD
    * DB_NAME
    * DB_ADDR

Now it's time to setup redis for server side caching.
Inside of platform/cache/redis.go change the neccesary data to hook up to redis

Finally you are able to run ```go run main.go``` to start the backend.
For this project to work you also need the frontend which is found at https://github.com/DedS3t/monopoly-frontend


