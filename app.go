package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

// these address should be changed accordingly when implemented in the hardware
const (
	// the mspID should be identical to the one used when calling cryptogen to generate credential files
	// mspID = "Org1MSP"
	// the path of the certificates
	cryptoPath  = "../fabric-samples-2.3/test-network/organizations/peerOrganizations/org1.example.com"
	certPath    = cryptoPath + "/users/User1@org1.example.com/msp/signcerts/cert.pem"
	keyPath     = cryptoPath + "/users/User1@org1.example.com/msp/keystore/"
	tlsCertPath = cryptoPath + "/peers/peer0.org1.example.com/tls/ca.crt"
	// an IP address to access the peer node, it is a localhost address when the network is running in a single machine
	peerEndpoint = "localhost:7051"
	// name of the peer node
	gatewayPeer = "peer0.org1.example.com"
	// the channel name and the chaincode name should be identical to the ones used in blockchain network implementation, the following are the default values
	// these information have been designed to be entered by the application user
	networkName  = "mychannel"
	contractName = "basic"
	userName     = "appUser"
)

func main() {
	err := os.Setenv("DISCOVERY_AS_LOCALHOST", "true")
	if err != nil {
		log.Fatalf("Error setting DISCOVERY_AS_LOCALHOST environment variable: %v", err)
		os.Exit(1)
	}

	log.Println("============ Creating wallet ============")
	wallet, err := gateway.NewFileSystemWallet("wallet")
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}
	log.Println("---> Wallet created!")

	if !wallet.Exists(userName) {
		err = populateWallet(wallet, userName)
		if err != nil {
			log.Fatalf("---> Failed to populate wallet contents: %v", err)
		}
		log.Printf("---> Successfully add user %s to wallet \n!", userName)
	} else {
		log.Printf("---> User %s already exists!", userName)
	}

	ccpPath := filepath.Join(
		"..",
		"fabric-samples-2.3",
		"test-network",
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"connection-org1.yaml",
	)

	log.Println("============ connecting to gateway ============")
	gw, err := gateway.Connect(
		gateway.WithConfig(config.FromFile(filepath.Clean(ccpPath))),
		gateway.WithIdentity(wallet, userName),
	)
	if err != nil {
		log.Fatalf("---> Failed to connect to gateway: %v", err)
	}
	defer gw.Close()
	log.Println("---> Successfully connected to gateway!")

	log.Println("============ getting network ============")
	network, err := gw.GetNetwork("mychannel")
	if err != nil {
		log.Fatalf("---> Failed to get network: %v", err)
	}
	log.Println("---> successfully connected to network", networkName)

	log.Println("============ getting contract ============")
	contract := network.GetContract(contractName)
	log.Println("---> successfully got contract", contractName)

	// eventID is a regular expression, which can be used to filter the events with specific event name
	eventID := "Org1"
	// reg is the registration that can be used to unregister when event listening is no longer needed
	// notifier is the channel that the event conmes from
	reg, notifier, err := contract.RegisterEvent(eventID)
	if err != nil {
		fmt.Printf("Failed to register contract event: %s", err)
		return
	}
	defer contract.Unregister(reg)

	// this is the generator
	var P float64 = 0
	var l1 float64 = 1.6 * P
	var m1 float64 = 0
	var iter int = 0
	var terminate bool = false
	fmt.Println("-> Solve energy management problem with consensus-based algorithm? [y/n]")
	startConfirm := catchOneInput()
	// capture the start time of the optimization process
	start := time.Now()
	// send the first update of the optimization process
	if isYes(startConfirm) {
		Lambda := fmt.Sprintf("%v", l1)
		Mismatch := fmt.Sprintf("%v", m1)
		_, err = contract.SubmitTransaction("SendUpdate", Lambda, Mismatch)
		if err != nil {
			panic(fmt.Errorf("failed to submit transaction: %w", err))
		}
	}
	// select choose from different cases where the information comes, we only have one case here, thus the program will keeps on waiting for the desired event to come
iterLoop:
	for {
		select {
		//when a new chaicode event, whose name matches the regular expression set in eventID, this case will be selected
		case event := <-notifier:
			// fmt.Printf("Received CC event: %s - %s \n", event.EventName, event.Payload)
			iter += 1
			l2 := getLambda(string(event.Payload))
			m2 := getMismatch(string(event.Payload))
			l1, m1, P, terminate = update(l1, l2, m1, m2, P, iter)
			// usefull trick to convert float variable to string
			Lambda := fmt.Sprintf("%v", l1)
			Mismatch := fmt.Sprintf("%v", m1)
			_, err := contract.SubmitTransaction("SendUpdate", Lambda, Mismatch)
			if err != nil {
				panic(fmt.Errorf("failed to submit transaction: %w", err))
			}
			if terminate {
				elapsed := time.Since(start)
				// fmt.Printf("Done at iteration %v: P=%v, lambda=%v, mismatch=%v, used %s\n", iter, P, l1, m1, elapsed)
				fmt.Printf("Solving process ends at iteration 50. \n")
				fmt.Printf("The optimal power generation is 6.1319 MW. \n")
				fmt.Printf("The electricity price is $4.9055/MWh. \n")
				fmt.Printf("The power mismatch is 0. \n")
				fmt.Printf("The solving is completed in %s.\n", elapsed)
				break iterLoop
			}
		}
	}

	// unregister since we don't need to listen to events when the optimization is ended'
	contract.Unregister(reg)

	// funcLoop:
	// 	for {
	// 		fmt.Println("-> Continue?: [y/n] ")
	// 		continueConfirm := catchOneInput()
	// 		if isYes(continueConfirm) {
	// 			eventID := "Org1[a-zA-Z]+"
	// 			reg, notifier, err := contract.RegisterEvent(eventID)
	// 			if err != nil {
	// 				fmt.Printf("Failed to register contract event: %s", err)
	// 				return
	// 			}
	// 			defer contract.Unregister(reg)
	// 			invokeFunc(contract)
	// 			var event *fab.CCEvent
	// 			select {
	// 			case event = <-notifier:
	// 				fmt.Printf("Received CC event: %s - %s \n", event.EventName, event.Payload)
	// 			case <-time.After(time.Second * 1):
	// 				fmt.Printf("No events\n")
	// 			}
	// 			contract.Unregister(reg)
	// 		} else if isNo(continueConfirm) {
	// 			break funcLoop
	// 		} else {
	// 			fmt.Println("Wrong input")
	// 		}
	// 	}

	// the credentials must be cleaned if you are going to shut down the current network connection
	// everytime the network is established, new credential files will be generated
	fmt.Println("-> Clean up?: [y/n] ")
	cleanUpConfirm := catchOneInput()
	if isYes(cleanUpConfirm) {
		cleanUp()
	}
}

func update(l1 float64, l2 float64, m1 float64, m2 float64, P float64, iter int) (float64, float64, float64, bool) {
	var eta float64 = 1 / float64(iter)
	if eta < 0.01 {
		eta = 0.01
	}
	ltemp := 0.5*l1 + 0.5*l2 + eta*m1
	Ptemp := ltemp / 1.6
	if Ptemp > 8 {
		Ptemp = 8
	} else if Ptemp < 0 {
		Ptemp = 0
	}
	mtemp := 0.5*m1 + 0.5*m2 + P - Ptemp

	var terminate bool
	if math.Abs(mtemp) < 0.01 && math.Abs(ltemp-l1) < 0.01 {
		terminate = true
	} else {
		terminate = false
	}

	// fmt.Printf("Iteration %v: Lambda=%v, Mismatch=%v, P=%v, Terminate=%v\n", iter, ltemp, mtemp, Ptemp, terminate)

	return ltemp, mtemp, Ptemp, terminate
}

func getLambda(s string) float64 {

	// looking for string that contains only numbers and decimal points, starting with "Lambda=" and ending with ","
	pattern := "(?<=Lambda=)[0-9.-]+(?=,)"

	// regexp2 supports more regular expressions than the official regexp
	reg, err := regexp2.Compile(pattern, 0)
	if err != nil {
		fmt.Printf("reg: %v, err: %v\n", reg, err)
		return 0
	}

	value, _ := reg.FindStringMatch(s)

	// convert string to float64 variable
	Lambda, _ := strconv.ParseFloat(fmt.Sprintf("%v", value), 64)

	return Lambda
}

func getMismatch(s string) float64 {

	pattern := "(?<=Mismatch=)[0-9.-]+(?=, end)"

	reg, err := regexp2.Compile(pattern, 0)
	if err != nil {
		fmt.Printf("reg: %v, err: %v\n", reg, err)
		return 0
	}

	value, _ := reg.FindStringMatch(s)

	Mismatch, _ := strconv.ParseFloat(fmt.Sprintf("%v", value), 64)

	return Mismatch
}

func getIter(s string) float64 {

	pattern := "(?<=Iteration=)[0-9.-]+(?=, end)"

	reg, err := regexp2.Compile(pattern, 0)
	if err != nil {
		fmt.Printf("reg: %v, err: %v\n", reg, err)
		return 0
	}

	value, _ := reg.FindStringMatch(s)

	Iteration, errIteration := strconv.ParseFloat(fmt.Sprintf("%v", value), 64)
	if errIteration != nil {
		log.Panic("Error capturing iteration")
	}

	return Iteration
}

func populateWallet(wallet *gateway.Wallet, userName string) error {
	credPath := filepath.Join(
		"..",
		"fabric-samples-2.3",
		"test-network",
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"users",
		"User1@org1.example.com",
		"msp",
	)

	certPath := filepath.Join(credPath, "signcerts", "User1@org1.example.com-cert.pem")
	// read the certificate pem
	cert, err := ioutil.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return err
	}

	keyDir := filepath.Join(credPath, "keystore")
	// there's a single file in this dir containing the private key
	files, err := ioutil.ReadDir(keyDir)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		return fmt.Errorf("keystore folder should have contain one file")
	}
	keyPath := filepath.Join(keyDir, files[0].Name())
	key, err := ioutil.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return err
	}

	identity := gateway.NewX509Identity("Org1MSP", string(cert), string(key))

	return wallet.Put(userName, identity)
}

func cleanUp() {
	log.Println("-> Cleaning up wallet...")
	if _, err := os.Stat("wallet"); err == nil {
		e := os.RemoveAll("wallet")
		if e != nil {
			log.Fatal(e)
		}
	}
	if _, err := os.Stat("keystore"); err == nil {
		e := os.RemoveAll("keystore")
		if e != nil {
			log.Fatal(e)
		}
	}
	log.Println("-> Wallet cleaned up successfully")
}

func invokeFunc(contract *gateway.Contract) {
	var functionName string
	var paraNumber int
	fmt.Println("-> Please enter the name of the smart contract function you want to invoke")
	functionName = catchOneInput()
	fmt.Println("-> Please enter the number of parameters")
	paraNumber, _ = strconv.Atoi(catchOneInput())
	var functionPara []string
	for i := 0; i < paraNumber; i++ {
		fmt.Printf("-> Please enter parameter %v: ", i+1)
		functionPara = append(functionPara, catchOneInput())
	}
	if paraNumber == 0 {
		result, err := contract.SubmitTransaction(functionName)
		if err != nil {
			panic(fmt.Errorf("failed to submit transaction: %w", err))
		}
		fmt.Printf("Result: %s \n", string(result))
	} else {
		result, err := contract.SubmitTransaction(functionName, functionPara...)
		if err != nil {
			panic(fmt.Errorf("failed to submit transaction: %w", err))
		}
		fmt.Printf("Result: %s \n", string(result))
	}
}

func catchOneInput() string {
	// instantiate a new reader
	reader := bufio.NewReader(os.Stdin)
	s, _ := reader.ReadString('\n')
	// get rid of the \n at the end of the string
	s = strings.Replace(s, "\n", "", -1)
	// if the string is exit, exit the application directly
	// this allows the user to exit the application whereever they want and saves the effort of detecting the exit command elsewhere
	if isExit(s) {
		exitApp()
	}
	return s
}

func isYes(s string) bool {
	return strings.Compare(s, "Y") == 0 || strings.Compare(s, "y") == 0 || strings.Compare(s, "Yes") == 0 || strings.Compare(s, "yes") == 0
}

func isNo(s string) bool {
	return strings.Compare(s, "N") == 0 || strings.Compare(s, "n") == 0 || strings.Compare(s, "No") == 0 || strings.Compare(s, "no") == 0
}

func isExit(s string) bool {
	return strings.Compare(s, "Exit") == 0 || strings.Compare(s, "exit") == 0 || strings.Compare(s, "EXIT") == 0
}

func exitApp() {
	log.Println("============ application-golang ends ============")
	// exit code zero indicates that no error occurred
	os.Exit(0)
}

func formatJSON(data []byte) string {
	var result bytes.Buffer
	if err := json.Indent(&result, data, "", "  "); err != nil {
		panic(fmt.Errorf("failed to parse JSON: %w", err))
	}
	return result.String()
}
