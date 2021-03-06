package proxy

import (
	"log"
	"regexp"
	"strings"

	"github.com/truechain/open-truechain-pool/rpc"
	"github.com/truechain/open-truechain-pool/util"
	"encoding/hex"
	"strconv"
)

// Allow only lowercase hexadecimal with 0x prefix
var noncePattern = regexp.MustCompile("^0x[0-9a-f]{16}$")
var hashPattern = regexp.MustCompile("^0x[0-9a-f]{64}$")
var workerPattern = regexp.MustCompile("^[0-9a-zA-Z-_]{1,8}$")

// Stratum
func (s *ProxyServer) handleLoginRPC(cs *Session, params []string, id string) (bool, *ErrorReply) {
	if len(params) == 0 {
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	login := strings.ToLower(params[0])
	if !util.IsValidHexAddress(login) {
		return false, &ErrorReply{Code: -1, Message: "Invalid login"}
	}
	if !s.policy.ApplyLoginPolicy(login, cs.ip) {
		return false, &ErrorReply{Code: -1, Message: "You are blacklisted"}
	}
	cs.login = login
	s.registerSession(cs)

	cs.worker = id
	log.Printf("Stratum miner connected %v@%v", login, cs.ip)
	return true, nil
}

func (s *ProxyServer) handleGetWorkRPC(cs *Session) ([]string, *ErrorReply) {

	var targetS string
	var Zeor []byte
	var ZeorTarge []byte
	var ft string

	t := s.currentBlockTemplate()
	if t == nil || len(t.Header) == 0 || s.isSick() {
		if t==nil{
			log.Println("----t is nill")
		}
		return nil, &ErrorReply{Code: 0, Message: "Work not ready"}
	}

	// block or fruit

	tarS := hex.EncodeToString(Starget.Bytes())

	for i:=0;i<32-len(tarS);i++{
		Zeor = append(Zeor,'0')
	}
	ztem := Zeor[:]
	tem3:= string(ztem)+tarS


	// if fruit tar less then starget so need use fruit tar to mine fruit
	if t.fTarget.Cmp(Starget)>0{
		var Zeor2 []byte
		for i:=0;i<32-len(hex.EncodeToString(t.fTarget.Bytes()));i++{
			Zeor2 = append(Zeor2,'0')
		}

		ft = string(Zeor2[:])+hex.EncodeToString(t.fTarget.Bytes())
	}


	for i:=0;i<32;i++{
		ZeorTarge = append(ZeorTarge,'0')
	}
	zore:=string(ZeorTarge[:])

	// 32(block)+32(fruit) Valid share from
	// 32(block)+32(fruit) Valid share from


	if t.fTarget.Uint64()== uint64(0){
		//block only
		targetS = "0x"+tem3+zore
	}else{
		if t.bTarget.Uint64()== uint64(0){
			//fruit only
			if t.fTarget.Cmp(Starget)<0{
				targetS = "0x"+zore+ft
				log.Println("----the is fruit taget","ftage",t.fTarget)
			}else{
				targetS = "0x"+zore+tem3
			}


		}else{
			// block and fruit
			if !t.iMinedFruit{
				if t.fTarget.Cmp(Starget)<0{
					targetS = "0x"+tem3+ft
				}else{
					targetS = "0x"+tem3+tem3
				}
			}else{
				targetS = "0x"+tem3+zore
			}

		}
	}

	log.Println("---work the len is","ft",len(ft),"tem3",len(tem3),"zore",len(zore),"tagrgets",len(targetS))

	return []string{t.Header, t.Seed, targetS}, nil
}

// Stratum
func (s *ProxyServer) handleTCPSubmitRPC(cs *Session, id string, params []string) (bool, *ErrorReply) {
	s.sessionsMu.RLock()
	_, ok := s.sessions[cs]
	s.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return s.handleSubmitRPC(cs, cs.login, id, params)
}

func (s *ProxyServer) handleSubmitRPC(cs *Session, login, id string, params []string) (bool, *ErrorReply) {
	if !workerPattern.MatchString(id) {
		id = "0"
	}
	if len(params) != 3 {
		s.policy.ApplyMalformedPolicy(cs.ip)
		log.Printf("Malformed params from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	/*
	if !noncePattern.MatchString(params[0]) || !hashPattern.MatchString(params[1]) || !hashPattern.MatchString(params[2]) {
		s.policy.ApplyMalformedPolicy(cs.ip)
		log.Printf("Malformed PoW result from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Malformed PoW result"}
	}*/
	t := s.currentBlockTemplate()
	exist, validShare := s.processShare(login, cs.worker, cs.ip, t, params)
	ok := s.policy.ApplySharePolicy(cs.ip, !exist && validShare)

	if exist {
		log.Printf("Duplicate share from %s@%s %v", login, cs.ip, params)
		return false, &ErrorReply{Code: 22, Message: "Duplicate share"}
	}

	if !validShare {
		log.Printf("Invalid share from %s@%s", login, cs.ip)
		// Bad shares limit reached, return error and close
		if !ok {
			return false, &ErrorReply{Code: 23, Message: "Invalid share"}
		}
		return false, nil
	}
//	log.Printf("Valid share from %s@%s", login, cs.ip)

	if !ok {
		return true, &ErrorReply{Code: -1, Message: "High rate of invalid shares"}
	}
	return true, nil
}

func (s *ProxyServer) handleGetHashRateRPC(cs *Session, params string){
	log.Println("-----hash","rate is",params)
	cs.hashrate , _ = strconv.ParseFloat(params,64)
	log.Println("-----hash2","rate is",cs.hashrate)
}

func (s *ProxyServer) handleGetBlockByNumberRPC() *rpc.GetBlockReplyPart {
	t := s.currentBlockTemplate()
	var reply *rpc.GetBlockReplyPart
	if t != nil {
		reply = t.GetPendingBlockCache
	}
	return reply
}

func (s *ProxyServer) handleUnknownRPC(cs *Session, m string) *ErrorReply {
	log.Printf("Unknown request method %s from %s", m, cs.ip)
	s.policy.ApplyMalformedPolicy(cs.ip)
	return &ErrorReply{Code: -3, Message: "Method not found"}
}
