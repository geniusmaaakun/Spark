package core

import (
	"Spark/modules"
	"crypto/rand"
	"encoding/hex"
	"errors"
	_net "net"
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"

	"github.com/denisbrodbeck/machineid"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

/*
システム情報を取得して modules.Device 構造体に格納する機能を提供します。具体的には、ローカルIPアドレス、MACアドレス、ネットワークIO情報、CPU、RAM、ディスク情報、デバイスの起動時間、ホスト名、ユーザー名などのシステム情報を収集します。


machineid.ProtectedID: デバイス固有の識別子を生成します。主にデバイスを一意に識別するために使用されます。
gopsutil ライブラリ: システムのCPU、メモリ、ディスク、ネットワーク、ホスト情報を取得するためのクロスプラットフォームライブラリです。
*/

/*
概要: プライベートIPアドレスかどうかを判断します。
仕組み: RFC1918で定義されたプライベートIPアドレス範囲（例: 10.0.0.0/8）に該当するかを確認します。
*/
func isPrivateIP(ip _net.IP) bool {
	var privateIPBlocks []*_net.IPNet
	for _, cidr := range []string{
		//"127.0.0.0/8",    // IPv4 loopback
		//"::1/128",        // IPv6 loopback
		//"fe80::/10",      // IPv6 link-local
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
	} {
		_, block, _ := _net.ParseCIDR(cidr)
		privateIPBlocks = append(privateIPBlocks, block)
	}
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

/*
概要: デバイスのローカルIPアドレスを取得します。
仕組み: ネットワークインターフェースを調べ、プライベートIPアドレスを持つものを返します。該当しない場合はエラーを返します。
*/
func GetLocalIP() (string, error) {
	ifaces, err := _net.Interfaces()
	if err != nil {
		return `<UNKNOWN>`, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return `<UNKNOWN>`, err
		}

		for _, addr := range addrs {
			var ip _net.IP
			switch v := addr.(type) {
			case *_net.IPNet:
				ip = v.IP
			case *_net.IPAddr:
				ip = v.IP
			}
			if isPrivateIP(ip) {
				if addr := ip.To4(); addr != nil {
					return addr.String(), nil
				} else if addr := ip.To16(); addr != nil {
					return addr.String(), nil
				}
			}
		}
	}
	return `<UNKNOWN>`, errors.New(`no IP address found`)
}

/*
概要: デバイスのMACアドレスを取得します。
仕組み: 使用可能なネットワークインターフェースのうち、最初のMACアドレスを取得して返します。
*/
func GetMacAddress() (string, error) {
	interfaces, err := _net.Interfaces()
	if err != nil {
		return ``, err
	}
	var address []string
	for _, i := range interfaces {
		a := i.HardwareAddr.String()
		if a != `` {
			address = append(address, a)
		}
	}
	if len(address) == 0 {
		return ``, nil
	}
	return strings.ToUpper(address[0]), nil
}

/*
概要: デバイスのネットワークIO情報を取得します。
仕組み: gopsutil ライブラリを使って、ネットワークの送受信バイト数を2回取得し、その差分を計算して返します。
*/
func GetNetIOInfo() (modules.Net, error) {
	result := modules.Net{}
	first, err := net.IOCounters(false)
	if err != nil {
		return result, nil
	}
	if len(first) == 0 {
		return result, errors.New(`failed to read network io counters`)
	}
	<-time.After(time.Second)
	second, err := net.IOCounters(false)
	if err != nil {
		return result, nil
	}
	if len(second) == 0 {
		return result, errors.New(`failed to read network io counters`)
	}
	result.Recv = second[0].BytesRecv - first[0].BytesRecv
	result.Sent = second[0].BytesSent - first[0].BytesSent
	return result, nil
}

/*
概要: デバイスのCPU情報を取得します。
仕組み: gopsutil を使って、CPUのモデル名、論理・物理コア数、使用率を取得します。CPUの使用率は3秒間のサンプリングを行い計算します。
*/
func GetCPUInfo() (modules.CPU, error) {
	result := modules.CPU{}
	info, err := cpu.Info()
	if err != nil {
		return result, nil
	}
	if len(info) == 0 {
		return result, errors.New(`failed to read cpu info`)
	}
	result.Model = info[0].ModelName
	result.Cores.Logical, _ = cpu.Counts(true)
	result.Cores.Physical, _ = cpu.Counts(false)
	stat, err := cpu.Percent(3*time.Second, false)
	if err != nil {
		return result, nil
	}
	if len(stat) == 0 {
		return result, errors.New(`failed to read cpu info`)
	}
	result.Usage = stat[0]
	return result, nil
}

/*
概要: デバイスのRAM情報を取得します。
仕組み: 使用中のメモリ量、全体のメモリ量、使用率を取得して返します。
*/
func GetRAMInfo() (modules.IO, error) {
	result := modules.IO{}
	stat, err := mem.VirtualMemory()
	if err != nil {
		return result, nil
	}
	result.Total = stat.Total
	result.Used = stat.Used
	result.Usage = float64(stat.Used) / float64(stat.Total) * 100
	return result, nil
}

/*
概要: デバイスのディスク使用情報を取得します。
仕組み: すべてのディスクパーティションを調べ、それぞれの使用量を合計して返します。
*/
func GetDiskInfo() (modules.IO, error) {
	devices := map[string]struct{}{}
	result := modules.IO{}
	disks, err := disk.Partitions(false)
	if err != nil {
		return result, nil
	}
	for i := 0; i < len(disks); i++ {
		if _, ok := devices[disks[i].Device]; !ok {
			devices[disks[i].Device] = struct{}{}
			stat, err := disk.Usage(disks[i].Mountpoint)
			if err == nil {
				result.Total += stat.Total
				result.Used += stat.Used
			}
		}
	}
	result.Usage = float64(result.Used) / float64(result.Total) * 100
	return result, nil
}

/*
概要: デバイスの詳細情報を取得して、modules.Device 構造体にまとめて返します。
収集する情報:
ID: デバイス固有のID。machineid ライブラリを使用して取得します。失敗した場合はランダムなIDを生成します。
ローカルIPアドレス、MACアドレス、CPU、ネットワークIO、RAM、ディスク使用量、起動時間、ホスト名、ユーザー名を取得し、まとめて返します。
*/
func GetDevice() (*modules.Device, error) {
	id, err := machineid.ProtectedID(`Spark`)
	if err != nil {
		id, err = machineid.ID()
		if err != nil {
			secBuffer := make([]byte, 16)
			rand.Reader.Read(secBuffer)
			id = hex.EncodeToString(secBuffer)
		}
	}
	localIP, err := GetLocalIP()
	if err != nil {
		localIP = `<UNKNOWN>`
	}
	macAddr, err := GetMacAddress()
	if err != nil {
		macAddr = `<UNKNOWN>`
	}
	cpuInfo, err := GetCPUInfo()
	if err != nil {
		cpuInfo = modules.CPU{
			Model: `<UNKNOWN>`,
			Usage: 0,
		}
	}
	netInfo, err := GetNetIOInfo()
	if err != nil {
		netInfo = modules.Net{
			Sent: 0,
			Recv: 0,
		}
	}
	ramInfo, err := GetRAMInfo()
	if err != nil {
		ramInfo = modules.IO{
			Total: 0,
			Used:  0,
			Usage: 0,
		}
	}
	diskInfo, err := GetDiskInfo()
	if err != nil {
		diskInfo = modules.IO{
			Total: 0,
			Used:  0,
			Usage: 0,
		}
	}
	uptime, err := host.Uptime()
	if err != nil {
		uptime = 0
	}
	hostname, err := os.Hostname()
	if err != nil {
		hostname = `<UNKNOWN>`
	}
	username, err := user.Current()
	if err != nil {
		username = &user.User{Username: `<UNKNOWN>`}
	} else {
		slashIndex := strings.Index(username.Username, `\`)
		if slashIndex > -1 && slashIndex+1 < len(username.Username) {
			username.Username = username.Username[slashIndex+1:]
		}
	}
	return &modules.Device{
		ID:       id,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		LAN:      localIP,
		MAC:      macAddr,
		CPU:      cpuInfo,
		RAM:      ramInfo,
		Net:      netInfo,
		Disk:     diskInfo,
		Uptime:   uptime,
		Hostname: hostname,
		Username: username.Username,
	}, nil
}

/*
概要: デバイスの部分的な情報（CPU、ネットワークIO、メモリ、ディスク使用量、起動時間）を取得します。GetDevice に比べ、少ない情報を返します。
*/
func GetPartialInfo() (*modules.Device, error) {
	cpuInfo, err := GetCPUInfo()
	if err != nil {
		cpuInfo = modules.CPU{
			Model: `<UNKNOWN>`,
			Usage: 0,
		}
	}
	netInfo, err := GetNetIOInfo()
	if err != nil {
		netInfo = modules.Net{
			Recv: 0,
			Sent: 0,
		}
	}
	memInfo, err := GetRAMInfo()
	if err != nil {
		memInfo = modules.IO{
			Total: 0,
			Used:  0,
			Usage: 0,
		}
	}
	diskInfo, err := GetDiskInfo()
	if err != nil {
		diskInfo = modules.IO{
			Total: 0,
			Used:  0,
			Usage: 0,
		}
	}
	uptime, err := host.Uptime()
	if err != nil {
		uptime = 0
	}
	return &modules.Device{
		Net:    netInfo,
		CPU:    cpuInfo,
		RAM:    memInfo,
		Disk:   diskInfo,
		Uptime: uptime,
	}, nil
}
