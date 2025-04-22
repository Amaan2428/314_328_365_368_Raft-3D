#Steps to run the project

1. Start the Cluster (3 Nodes)
Open three PowerShell terminals and run:

Node 1 (bootstrap):

go run main.go --id=node1 --data-dir=raft_data_node1 --raft-port=65362 --http-port=8080 --bootstrap=true

Node 2:

go run main.go --id=node2 --data-dir=raft_data_node2 --raft-port=65363 --http-port=8081 --bootstrap=false

Node 3:
go run main.go --id=node3 --data-dir=raft_data_node3 --raft-port=65364 --http-port=8082 --bootstrap=false

2. Check Cluster Health and Leadership
Open a new PowerShell terminal and run:

Invoke-WebRequest -Uri "http://localhost:8080/admin/status"
Invoke-WebRequest -Uri "http://localhost:8081/admin/status"
Invoke-WebRequest -Uri "http://localhost:8082/admin/status"

Invoke-WebRequest -Uri "http://localhost:8080/admin/leader"
Invoke-WebRequest -Uri "http://localhost:8081/admin/leader"
Invoke-WebRequest -Uri "http://localhost:8082/admin/leader"

3. Add a Printer

$body = @{
    id = "p1"
    company = "Prusa"
    model = "MK3S"
} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/printers" -Method POST -Body $body -ContentType "application/json"

4. Add a Filament

$body = @{
    id = "f1"
    type = "PLA"
    color = "Red"
    total_weight_in_grams = 1000
} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/filaments" -Method POST -Body $body -ContentType "application/json"

5. Create a Print Job

$body = @{
    id = "j1"
    printer_id = "p1"
    filament_id = "f1"
    filepath = "/prints/part.gcode"
    print_weight_in_grams = 100
} | ConvertTo-Json
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/print_jobs" -Method POST -Body $body -ContentType "application/json"

6. List All Print Jobs

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/print_jobs"

7. Update Print Job Status
Queued → Running:

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/print_jobs/j1/status?status=Running" -Method POST

Running → Done:

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/print_jobs/j1/status?status=Done" -Method POST

8. List Filaments and Verify Weight Deduction

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/filaments"

9. List Printers

Invoke-RestMethod -Uri "http://localhost:8080/api/v1/printers"

10. Failover Demo
Stop (Ctrl+C) the leader node.
Wait a few seconds, then check Check Cluster Health and Leadership from (2) on the other nodes to see the new leader.
Continue sending POST/GET requests to the new leader.

11. Recovery Demo
Restart the killed node with its original data directory:

go run main.go --id=node1 --data-dir=raft_data_node1 --raft-port=65362 --http-port=8080

It will rejoin as a follower and catch up with the cluster.
