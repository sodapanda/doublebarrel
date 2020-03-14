package main

//todo 弃用自带的服务器，自带的服务器没有线程池，自动开线程导致没法细化处理,每个线程有自己的dnsClient
//todo 使用Rx 进行双请求同时发出
//todo Trie改成binary key，不再使用string 减少内存使用,tree高度优化 DONE
//todo 部分国内域名对ECS支持不好，比如百度会解析出香港地址
//todo signal的处理
//todo log保存和处理
//todo 提供配置文件 done
//todo 对接上级socks5代理
//todo 提供systemd脚本和运行时的user配置

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/bluele/gcache"
	"github.com/miekg/dns"
	"github.com/yl2chen/cidranger"
)

var cacheData gcache.Cache
var ranger cidranger.Ranger

var configPath string
var mConfig config

const version = "0.0.1"

type config struct {
	Cache          int
	LocalPublicIP  string
	RemotePublicIP string
	Listen         string
	DNSServer      string
	NetRange       string
	Forward        []struct {
		Domain string
		Server string
	}
}

func checkCache(name string) (*dns.Msg, error) {
	if item, err := cacheData.Get(name); err == nil {
		return item.(*dns.Msg), nil
	}
	return nil, errors.New("cache key not found")
}

func addCache(name string, item *dns.Msg) {
	ttl := item.Answer[len(item.Answer)-1].Header().Ttl
	cacheData.SetWithExpire(name, item, time.Second*time.Duration(ttl))
}

func query(request *dns.Msg, subnet string, server string, tls bool, ecs bool) (*dns.Msg, error) {
	if ecs {
		o := &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
			},
		}

		e := &dns.EDNS0_SUBNET{
			Code:          dns.EDNS0SUBNET,
			Address:       net.ParseIP(subnet),
			Family:        1, // IP4
			SourceNetmask: net.IPv4len * 8,
		}

		o.Option = append(o.Option, e)
		request.Extra = append(request.Extra, o)
	}

	client := new(dns.Client)
	if tls {
		client.Net = "tcp-tls"
	} else {
		client.Net = "udp"
	}
	in, _, err := client.Exchange(request, server)

	if err != nil {
		fmt.Println("ERROR:", err)
		return nil, err
	}

	return in, nil
}

func serve() {
	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		name := r.Question[0].Name
		//check local
		isForward := handleForward(name, r, w)
		if isForward {
			return
		}
		//check query
		if cacheValue, err := checkCache(name); err == nil {
			cacheValue.SetReply(r)
			w.WriteMsg(cacheValue)
			log(name, true, false, false)
			return
		}

		//china query
		rstLocal, err := query(r, mConfig.LocalPublicIP, mConfig.DNSServer, true, true)
		if err != nil {
			return
		}
		if len(rstLocal.Answer) == 0 {
			fmt.Println(name + " no answer!")
			w.WriteMsg(rstLocal)
			return
		}
		localIP, ok := rstLocal.Answer[len(rstLocal.Answer)-1].(*dns.A)
		if !ok {
			fmt.Println(name + " last record is not A")
			w.WriteMsg(rstLocal)
			return
		}
		isChinaL := checkChina(localIP.A)
		if isChinaL {
			w.WriteMsg(rstLocal)
			addCache(name, rstLocal)
			log(name, false, false, true)
			return
		}
		//world query
		rstRemote, err := query(r, mConfig.RemotePublicIP, mConfig.DNSServer, true, true)
		if err != nil {
			return
		}
		w.WriteMsg(rstRemote)
		addCache(name, rstRemote)
		log(name, false, false, false)
	})

	srv := &dns.Server{Addr: mConfig.Listen, Net: "udp"}
	err := srv.ListenAndServe()
	fmt.Println(err)
}

func log(domain string, hitCache bool, isLocal bool, isChina bool) {
	logRst := domain + "|"
	if hitCache {
		logRst += "cache hit"
		fmt.Println(logRst)
		return
	}
	if isLocal {
		logRst += "locale domain"
		fmt.Println(logRst)
		return
	}
	if isChina {
		logRst += "C"
	} else {
		logRst += "W"
	}

	fmt.Println(logRst)
}

func handleForward(name string, request *dns.Msg, w dns.ResponseWriter) bool {
	isForward := false
	serverToQuery := ""
	if len(mConfig.Forward) <= 0 {
		return false
	}

	for _, item := range mConfig.Forward {
		if name == item.Domain+"." || strings.HasSuffix(name, "."+item.Domain+".") {
			isForward = true
			serverToQuery = item.Server
			break
		}
	}

	if !isForward {
		return false
	}
	fmt.Println(name, "to forward to ", serverToQuery)
	rst, err := query(request, "", serverToQuery, false, false)
	if err != nil {
		fmt.Println(err)
		return true
	}
	w.WriteMsg(rst)

	return true
}

func loadNetRange() error {
	fileBytes, err := ioutil.ReadFile(mConfig.NetRange)
	if err != nil {
		return err
	}
	ipSlice := strings.Split(string(fileBytes), "\n")

	ranger = cidranger.NewPCTrieRanger()

	for _, item := range ipSlice {
		_, network, _ := net.ParseCIDR(item)
		ranger.Insert(cidranger.NewBasicRangerEntry(*network))
	}
	return nil
}

func checkChina(address net.IP) bool {
	contains, err := ranger.Contains(address)
	if err != nil {
		fmt.Println(err)
		return false
	}
	return contains
}

func readConfig() error {
	file, err := os.Open(configPath)
	defer file.Close()
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(file)
	mConfig = config{}
	decodeErr := decoder.Decode(&mConfig)
	if decodeErr != nil {
		return decodeErr
	}
	if len(mConfig.LocalPublicIP) == 0 {
		return errors.New("locale public ip not found in config")
	}
	if len(mConfig.RemotePublicIP) == 0 {
		return errors.New("remote public ip not found in config")
	}
	if len(mConfig.LocalPublicIP) == 0 {
		return errors.New("remote public ip not found in config")
	}
	if len(mConfig.Listen) == 0 {
		return errors.New("listen address not found in config")
	}
	if len(mConfig.DNSServer) == 0 {
		return errors.New("dnsServer address not found in config")
	}
	if len(mConfig.NetRange) == 0 {
		return errors.New("netRange not found in config")
	}
	return nil
}

func readFlag() error {
	flagConfigPath := flag.String("config", "config.json", "path of config file")
	flagVersion := flag.Bool("v", false, "get version")

	flag.Parse()
	configPath = *flagConfigPath

	if *flagVersion {
		return errors.New("only version")
	}
	return nil
}

func main() {
	flagErr := readFlag()
	if flagErr != nil {
		fmt.Println("Version:", version)
		return
	}
	configError := readConfig()
	if configError != nil {
		fmt.Println(configError)
		return
	}

	cacheData = gcache.New(mConfig.Cache).LRU().Build()
	fmt.Println("start listening on ", mConfig.Listen)

	netRangeErr := loadNetRange()
	if netRangeErr != nil {
		fmt.Println(netRangeErr)
		return
	}

	serve()
}
