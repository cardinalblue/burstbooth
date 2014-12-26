Burstbooth
-----

### Create tables in DynamoDB local
Run `make localddb`.

### Start local server
Run `make local`.

### Create elasticbeanstalk zip file
Run `make ec2`

### Talking to DynamoDB local
DynamoDB local has bug that double base encodes byte slice attributes.
Use the following curl template to talk to DynamoDB local instead.
```
curl -XPOST -H 'X-Amz-Target: DynamoDB_20120810.UpdateItem' -H 'Authorization: AWS4-HMAC-SHA256 Credential=BurstboothDev/20141226/us-east-1/localhost:8000/aws4_request' -d '{"TableName":"Post","Key":{"I":{"S":"gif"},"K":{"B":"E7NkXQvfTSo="}},"UpdateExpression":"set S = :s","ExpressionAttributeValues":{":s":{"N":"8"}},"ReturnValues":"ALL_NEW"}'  http://127.0.0.1:8000/
```
