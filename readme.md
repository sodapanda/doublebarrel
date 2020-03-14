# Double Barrel

Issue two DNS requests concurrently.Get the best CDN optimized result without leaking your query. It means nobody on the wire konws what you quered for.You can choose DNS servers that you trust.

![icon](https://raw.githubusercontent.com/sodapanda/doublebarrel/master/icon.png "Logo Title Text 1")

## Features

If you live in China or Iran, you might be an expert of network proxy. when you turn on your proxy client, All network traffic is forwarded by the proxy server, even websites that have servers in your country which don't need to be proxied. You can solve this problem by adding routing table rules in your router, but there is something seems tricky:the DNS. If you got an wrong IP of a domain name, everything else will not be right. Big companies have CDN servers in every country. If you use the results returned by the proxy server blindly, the traffic that should have directly routed to the local CDN server may also be mistakenly forwarded to the proxy server.
This project exists to solve this problem. Double barrel sends out two dns requests at the same time, and the ECS fields indicate the IP that your locale ISP gives to you and the IP of your proxy server.After returning the result, make a comparison and choose the best answer.With DNS-over-tls on no body knows what you have queried for. Congratulations , you are not going to jail.

## Installation

Download a binary version or build from source. 

## Usages

You need two config files. 

__config.json:__

```js
    {
    "cache": 3000, // cache size
    "localPublicIP": "115.195.37.189", //IP address that your ISP gives to you.You can find it by visiting https://www.ipip.net/  used for ECS
    "remotePublicIP": "112.118.253.82", // IP address of your proxy server. Used for ECS
    "listen": ":53", // UDP port to listen on,you change your OS's DNS server to this address
    "dnsServer": "8.8.8.8:853", // Upstream server to handle your request, the server must supoort DNS-over-tls
    "netRange": "cidrlist", // CIDR range list of your country(The one without freedom.)
    "forward": [ //You want to use a specific dns server to resolve some domain names, such as your company's intranet domain name.
        {
            "domain": "baidu.com",
            "server": "223.5.5.5:53"
        }
    ]
}
```

__cidrlist:__

    20.139.160.0/20
    20.249.255.0/24
    20.251.0.0/22
    23.236.64.0/25
    23.236.64.128/26
    23.236.64.192/27
    27.0.128.0/21
    .......

__run__

./doublebarrel -config config.json

or with sudo

## Acknowledgements

based on :

https://github.com/miekg/dns