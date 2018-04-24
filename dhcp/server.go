package sabakan

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"

	"github.com/cybozu-go/log"
	"go.universe.tf/netboot/dhcp4"
)

// Server is a DHCP server
type Server interface {
	Serve(ctx context.Context) error
	Close() error
}

// New creates a new dhcp Server object
func New(bind string, ifname string, ipxe string, begin, end net.IP) (Server, error) {
	conn, err := dhcp4.NewConn(bind)
	if err != nil {
		return nil, err
	}

	s := new(dhcpserver)
	s.conn = conn
	s.ifname = ifname
	s.ipxe = ipxe
	s.begin = begin
	s.end = end
	s.leases = make(map[uint32]struct{})
	return s, nil
}

type dhcpserver struct {
	conn *dhcp4.Conn

	ifname string
	ipxe   string

	begin  net.IP
	end    net.IP
	leases map[uint32]struct{}
}

// Architecture represent an architecture type
type Architecture int

// Architecture type
const (
	ArchIA32 Architecture = iota
	ArchX64
)

// Firmware represent a firmware type
type Firmware int

// Firmware type
const (
	FirmwareX86PC Firmware = iota
	FirmwareEFI32
	FirmwareEFI64
	FirmwareEFIBC
	FirmwareX86Ipxe
)

func (s *dhcpserver) handleDiscover(ctx context.Context, pkt *dhcp4.Packet, intf *net.Interface) error {
	fmt.Printf("isBootDHCP: %v\n", s.isBootDHCP(pkt))
	/*
		if err = s.isBootDHCP(pkt); err != nil {
			log.Debug("DHCP: Ignoring packet", map[string]interface{}{
				"mac_address": pkt.HardwareAddr,
				"error":       err,
			})
			continue
		}
	*/
	arch, fwtype, err := s.validateDHCP(pkt)
	fmt.Printf("arch: %v, fwtype: %v, err: %v\n", arch, fwtype, err)
	/*
		if err != nil {
			log.Debug("DHCP: Unusable packet", map[string]interface{}{
				"mac_address": pkt.HardwareAddr,
				"error":       err,
			})
			continue
		}
	*/

	log.Debug("DHCP: Got valid request to boot", map[string]interface{}{
		"mac_address": pkt.HardwareAddr,
		"error":       err,
	})
	log.Debug("DHCP: Got valid request", map[string]interface{}{
		"mac_address":  pkt.HardwareAddr,
		"architecture": arch,
	})

	ip, err := s.nextIPAddress()
	fmt.Printf("nextIPAddress: %v, err: %v\n", ip, err)
	if err != nil {
		log.Info("DHCP: Couldn't allocate ip address", map[string]interface{}{
			"mac_address": pkt.HardwareAddr,
			"error":       err,
		})
		return nil
	}

	serverIP, err := interfaceIP(intf)
	fmt.Printf("interfaceIP: %v, err: %v\n", serverIP, err)
	if err != nil {
		log.Info("DHCP: Couldn't get a source address", map[string]interface{}{
			"mac_address": pkt.HardwareAddr,
			"interface":   intf.Name,
			"error":       err,
		})
		return nil
	}

	resp, err := s.offerDHCP(pkt, serverIP, arch, fwtype, ip)
	if err != nil {
		log.Info("DHCP: Failed to construct ProxyDHCP offer", map[string]interface{}{
			"mac_address": pkt.HardwareAddr,
			"error":       err,
		})
		return nil
	}

	if err = s.conn.SendDHCP(resp, intf); err != nil {
		log.Info("DHCP: Failed to send ProxyDHCP offer", map[string]interface{}{
			"mac_address": pkt.HardwareAddr,
			"error":       err,
		})
		return nil
	}

	return nil
}

func (s *dhcpserver) handleRequest(ctx context.Context, pkt *dhcp4.Packet, intf *net.Interface) error {

	ip := pkt.Options[dhcp4.OptRequestedIP]
	fmt.Printf("requested ip: %v\n", ip)

	serverIP, err := interfaceIP(intf)
	fmt.Printf("interfaceIP: %v, err: %v\n", serverIP, err)

	resp, err := s.ackDHCP(pkt, serverIP, ip)
	if err != nil {
		log.Info("DHCP: Failed to construct ProxyDHCP ack", map[string]interface{}{
			"mac_address": pkt.HardwareAddr,
			"error":       err,
		})
		return nil
	}

	if err = s.conn.SendDHCP(resp, intf); err != nil {
		log.Info("DHCP: Failed to send ProxyDHCP ack", map[string]interface{}{
			"mac_address": pkt.HardwareAddr,
			"error":       err,
		})
		return nil
	}
	return nil
}

func (s *dhcpserver) Serve(ctx context.Context) error {
	for {
		pkt, intf, err := s.conn.RecvDHCP()
		if err != nil {
			return fmt.Errorf("Receiving DHCP packet: %s", err)
		}
		fmt.Printf("received dhcp packet: %s\n", intf.Name)
		/*
			if intf.Name != s.ifname {
				log.Debug("DHCP: Ignoring packet", map[string]interface{}{
					"listen_interface": s.ifname,
					"received_on":      intf.Name,
				})
				continue
			}
		*/

		switch pkt.Type {
		case dhcp4.MsgDiscover:
			err = s.handleDiscover(ctx, pkt, intf)
		case dhcp4.MsgRequest:
			err = s.handleRequest(ctx, pkt, intf)
		default:
			err = fmt.Errorf("unknown packet type: %v", pkt.Type)
		}

		if err != nil {
			return err
		}

	}
}

func (s *dhcpserver) isBootDHCP(pkt *dhcp4.Packet) error {
	if pkt.Type != dhcp4.MsgDiscover {
		return fmt.Errorf("packet is %s, not %s", pkt.Type, dhcp4.MsgDiscover)
	}

	if pkt.Options[93] == nil {
		return errors.New("not a PXE boot request (missing option 93)")
	}

	return nil
}

func (s *dhcpserver) validateDHCP(pkt *dhcp4.Packet) (arch Architecture, fwtype Firmware, err error) {
	fwt, err := pkt.Options.Uint16(93)
	if err != nil {
		return 0, 0, fmt.Errorf("malformed DHCP option 93 (required for PXE): %s", err)
	}

	switch fwt {
	case 0:
		arch = ArchIA32
		fwtype = FirmwareX86PC
	case 6:
		arch = ArchIA32
		fwtype = FirmwareEFI32
	case 7:
		arch = ArchX64
		fwtype = FirmwareEFI64
	case 9:
		arch = ArchX64
		fwtype = FirmwareEFIBC
	default:
		return 0, 0, fmt.Errorf("unsupported client firmware type '%d'", fwtype)
	}

	if class, err := pkt.Options.String(77); err == nil {
		if class == "iPXE" && fwtype == FirmwareX86PC {
			fwtype = FirmwareX86Ipxe
		}
	}
	return arch, fwtype, nil
}

func (s *dhcpserver) offerDHCP(pkt *dhcp4.Packet, serverIP net.IP, arch Architecture, fwtype Firmware, clientIP net.IP) (*dhcp4.Packet, error) {
	resp := &dhcp4.Packet{
		Type:          dhcp4.MsgOffer,
		TransactionID: pkt.TransactionID,
		Broadcast:     true,
		HardwareAddr:  pkt.HardwareAddr,
		RelayAddr:     pkt.RelayAddr,
		ServerAddr:    serverIP,
		ClientAddr:    nil,
		YourAddr:      clientIP,
		Options:       make(dhcp4.Options),
	}
	resp.Options[dhcp4.OptServerIdentifier] = serverIP
	resp.Options[dhcp4.OptVendorIdentifier] = []byte("HTTPClient")
	resp.Options[97] = pkt.Options[97]

	switch fwtype {
	case FirmwareEFI32, FirmwareEFI64, FirmwareEFIBC:
		resp.BootServerName = serverIP.String()
		resp.BootFilename = s.ipxe
	default:
		resp.BootServerName = serverIP.String()
		resp.BootFilename = s.ipxe
		//return nil, fmt.Errorf("unknown firmware type %d", fwtype)
	}

	return resp, nil
}

func (s *dhcpserver) ackDHCP(pkt *dhcp4.Packet, serverIP net.IP, clientIP net.IP) (*dhcp4.Packet, error) {
	resp := &dhcp4.Packet{
		Type:          dhcp4.MsgAck,
		TransactionID: pkt.TransactionID,
		Broadcast:     true,
		HardwareAddr:  pkt.HardwareAddr,
		RelayAddr:     pkt.RelayAddr,
		ServerAddr:    serverIP,
		ClientAddr:    nil,
		YourAddr:      clientIP,
		Options:       make(dhcp4.Options),
	}

	resp.Options[dhcp4.OptDHCPMessageType] = []byte{5}

	return resp, nil
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}

func (s *dhcpserver) nextIPAddress() (net.IP, error) {
	ibegin := ip2int(s.begin)
	iend := ip2int(s.end)
	fmt.Printf("begin: %v, end:%v\n", s.begin, s.end)
	for n := ibegin; n <= iend; n++ {
		if _, ok := s.leases[n]; ok {
			continue
		}

		s.leases[n] = struct{}{}

		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, n)
		return ip, nil
	}
	return nil, errors.New("leases are full")
}

func interfaceIP(intf *net.Interface) (net.IP, error) {
	addrs, err := intf.Addrs()
	if err != nil {
		return nil, err
	}

	// Try to find an IPv4 address to use, in the following order:
	// global unicast (includes rfc1918), link-local unicast,
	// loopback.
	fs := [](func(net.IP) bool){
		net.IP.IsGlobalUnicast,
		net.IP.IsLinkLocalUnicast,
		net.IP.IsLoopback,
	}
	for _, f := range fs {
		for _, a := range addrs {
			ipaddr, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipaddr.IP.To4()
			if ip == nil {
				continue
			}
			if f(ip) {
				return ip, nil
			}
		}
	}

	return nil, errors.New("no usable unicast address configured on interface")
}

func (s *dhcpserver) Close() error {
	return s.conn.Close()
}
