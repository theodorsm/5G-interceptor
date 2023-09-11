A go application to intercept and modify 5G NAS messages, to be used with a modified version of open5gs. Yaml files are used to define testcases to be run. Results is stored in mongodb.

## Configuration

Example configuration can be found in `testcases_example.yaml`. When running the testcases, `testcases.yaml` will be used.


Explanation:
```yaml
supi: "imsi-999700000000001" # supi/imsi of UE to intercept

testcases:
  - 
    id: 1 # Id that can be used to easly identiy case in database.
    plain: true #  If true, the intercepted message will be modified and returned before encryption and integrity protection is applied. Otherwise, modifcation of messages will happen after.
    msg_type: 93 # Message type to intercept. 93 = Security Mode Command
    integrity: "2" # Integrity algorithm in Security Mode Command message
    ciphering: "1" # Ciphering algorithm in Security Mode Command message
  - 
    id: 2
    plain: true
    msg_type: 93 
    integrity: "1"
    ciphering: "2"
    mac: "00000000" # MAC of message, must be of length 8
  - 
    id: 3
    plain: false
    msg_type: 93 
    offsets:
      - offset: 20 # Index of hex-string representation of message. 20 is the offset of selected ciphering/integrity alg
        value:  "01" # Value to replace original field. Hex-string representation of byte (two char = 1 byte)
      - offset: 4 # Offset of MAC
        value: "00000000"

```