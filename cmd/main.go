package main

import (
    "log"
    "net"
    "sync"
    "github.com/goburrow/modbus"
)

// PLC holds registers and coils
type PLC struct {
    coils     []bool
    registers []uint16
    mu        sync.Mutex
}

// NewPLC creates a new PLC with specified size
func NewPLC(numCoils, numRegisters int) *PLC {
    return &PLC{
        coils:     make([]bool, numCoils),
        registers: make([]uint16, numRegisters),
    }
}

// ModbusHandler handles Modbus requests
type ModbusHandler struct {
    plc *PLC
}

// NewModbusHandler creates a new Modbus handler
func NewModbusHandler(plc *PLC) *ModbusHandler {
    return &ModbusHandler{plc: plc}
}

// HandleRequest processes Modbus requests
func (h *ModbusHandler) HandleRequest(req modbus.Request) (res modbus.Response, err error) {
    h.plc.mu.Lock()
    defer h.plc.mu.Unlock()

    switch pdu := req.(type) {
    case *modbus.ReadCoilsRequest:
        data := make([]byte, (pdu.Quantity+7)/8)
        for i := uint16(0); i < pdu.Quantity; i++ {
            if h.plc.coils[pdu.Address+i] {
                data[i/8] |= 1 << (i % 8)
            }
        }
        res = &modbus.ReadCoilsResponse{Value: data}
    case *modbus.ReadHoldingRegistersRequest:
        data := make([]byte, pdu.Quantity*2)
        for i := uint16(0); i < pdu.Quantity; i++ {
            offset := i * 2
            value := h.plc.registers[pdu.Address+i]
            data[offset] = byte(value >> 8)
            data[offset+1] = byte(value)
        }
        res = &modbus.ReadHoldingRegistersResponse{Value: data}
    case *modbus.WriteSingleCoilRequest:
        h.plc.coils[pdu.Address] = pdu.Value
        res = &modbus.WriteSingleCoilResponse{Address: pdu.Address, Value: pdu.Value}
    case *modbus.WriteSingleRegisterRequest:
        h.plc.registers[pdu.Address] = pdu.Value
        res = &modbus.WriteSingleRegisterResponse{Address: pdu.Address, Value: pdu.Value}
    default:
        err = modbus.ExceptionCodeIllegalFunction
    }
    return
}

func main() {
    plc := NewPLC(100, 100) // Initialize PLC with 100 coils and 100 registers
    handler := NewModbusHandler(plc)

    listener, err := net.Listen("tcp", "0.0.0.0:502")
    if err != nil {
        log.Fatalf("Failed to bind to port 502: %v", err)
    }
    defer listener.Close()

    log.Println("PLC Simulator is running on port 502")

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Printf("Failed to accept connection: %v", err)
            continue
        }

        go func(conn net.Conn) {
            defer conn.Close()
            log.Printf("Accepted connection from %v", conn.RemoteAddr())

            modbusServer := modbus.NewTCPServer(conn)
            for {
                req, err := modbusServer.ReadRequest()
                if err != nil {
                    log.Printf("Failed to read request: %v", err)
                    return
                }

                res, err := handler.HandleRequest(req)
                if err != nil {
                    log.Printf("Failed to handle request: %v", err)
                    return
                }

                err = modbusServer.WriteResponse(res)
                if err != nil {
                    log.Printf("Failed to write response: %v", err)
                    return
                }
            }
        }(conn)
    }
}

