#!/bin/bash

# Start server in background
./server &
SERVER_PID=$!
sleep 2

echo "--- Testing Import ---"
curl -F "file=@players.csv" http://localhost:8080/api/players/import
echo ""

echo "--- Testing List Players ---"
curl http://localhost:8080/api/players
echo ""

echo "--- Testing Create Flight ---"
FLIGHT_RESP=$(curl -s -X POST -d '{"name":"Flight A"}' http://localhost:8080/api/flights)
echo $FLIGHT_RESP
FLIGHT_ID=$(echo $FLIGHT_RESP | jq -r '.id')
echo "Flight ID: $FLIGHT_ID"

echo "--- Testing Assign Player ---"
# Assign player 1 to flight
curl -X POST -d "{\"flight_id\":$FLIGHT_ID, \"player_id\":1}" http://localhost:8080/api/flights/assign
echo ""

echo "--- Testing List Flights ---"
curl http://localhost:8080/api/flights
echo ""

echo "--- Testing Submit Score ---"
# Player 1, Hole 1, Score 4
curl -X POST -d '{"player_id":1, "hole_number":1, "strokes":4}' http://localhost:8080/api/scores
echo ""
# Player 1, Hole 2, Score 12 (should be capped at 11)
curl -X POST -d '{"player_id":1, "hole_number":2, "strokes":12}' http://localhost:8080/api/scores
echo ""

echo "--- Testing Get Scores ---"
curl "http://localhost:8080/api/scores?player_id=1"
echo ""

echo "--- Testing Results ---"
curl http://localhost:8080/api/results
echo ""

# Kill server
kill $SERVER_PID
rm tournament.db
