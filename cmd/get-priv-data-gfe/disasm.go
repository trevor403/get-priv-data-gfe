package main

import (
	"bytes"
	"container/list"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io/ioutil"
	"log"
	"os"

	"github.com/pkg/errors"
	"golang.org/x/arch/x86/x86asm"
)

const streamPath = "C:\\Program Files\\NVIDIA Corporation\\ShadowPlay\\NVSPCAPS\\_nvspcaps64.dll"

const steamChecksum uint32 = 0x85ac72fb
const nvidiaChecksum uint32 = 0x3806c005

var validChecksums = []uint32{steamChecksum, nvidiaChecksum}

func getTextSection(buf []byte) (*pe.Section, int, uint64) {
	reader := bytes.NewReader(buf)

	file, err := pe.NewFile(reader)
	if err != nil {
		log.Fatal(err)
	}

	var arch int = 64
	if file.FileHeader.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		log.Fatalf("support for machine architecture %v not yet implemented", file.FileHeader.Machine)
	}

	//BaseOfData - Size - Offset

	var sectionOffset uint64
	opt, ok := file.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		panic(fmt.Errorf("support for optional header type %T not yet implemented", file.OptionalHeader))
	}

	sectionOffset += uint64(opt.ImageBase)
	sectionOffset += uint64(opt.BaseOfCode)

	var section *pe.Section
	for _, sec := range file.Sections {
		if sec.Name == ".text" {
			section = sec
		}
	}
	if section == nil {
		log.Fatal("could not find text section")
	}

	// fmt.Printf("HEADER\t opt.ImageBase = %x\n", uint64(opt.ImageBase))
	// fmt.Printf("HEADER\t opt.BaseOfCode = %x\n", uint64(opt.BaseOfCode))
	// fmt.Printf("HEADER\t section.Size = %x\n", uint64(section.Size))
	// fmt.Printf("HEADER\t section.Offset = %x\n", uint64(section.Offset))

	// fmt.Printf("%# v\n", pretty.Formatter(section))

	return section, arch, sectionOffset
}

func checkSizeAssigment(inst *x86asm.Inst) bool {
	// fmt.Println(inst.Opcode)
	// fmt.Printf("%# v\n", pretty.Formatter(inst))

	mem, typeOk := inst.Args[0].(x86asm.Mem)
	if !typeOk {
		return false
	}
	imm, typeOk := inst.Args[1].(x86asm.Imm)
	if !typeOk {
		return false
	}

	// the C code looks something like this
	//   uint8 privData[16];
	//   ...
	//   NvFBCCreateParams createParams;
	//   memset(&createParams, 0, sizeof(createParams));
	//   ...
	//   ((&createParams.pPrivateData) + 0x0) = 0xAABBCCDD;
	//   ((&createParams.pPrivateData) + 0x4) = 0xEEFF0011;
	//   ((&createParams.pPrivateData) + 0x8) = 0x22334455;
	//   ((&createParams.pPrivateData) + 0x10) = 0x66778899;
	//   createParams.dwPrivateDataSize = 16;

	// so we want to find an asignment assignment (MOV dword ptr) of value 16 (10h)
	// proceeded by 4x evenly spaces 4 byte assignments

	if inst.Op == x86asm.MOV && inst.Opcode>>16 == 0xc744 && imm == 0x10 && mem.Scale == 0x1 {
		// fmt.Println("got ya ========================")
		// fmt.Printf("%# v\n", pretty.Formatter(prev))
		// fmt.Printf("%# v\n", pretty.Formatter(inst))
		return true
	}

	return false
}

func extractFromInstList(instList *list.List) []byte {
	data := make([]byte, 16)

	partCount := 0
	var structAddr int64 = -1
	for e := instList.Front(); e != nil; e = e.Next() {
		inst := e.Value.(x86asm.Inst)

		mem, typeOk := inst.Args[0].(x86asm.Mem)
		if !typeOk {
			// log.Fatal("bad type mem")
			continue
		}
		imm, typeOk := inst.Args[1].(x86asm.Imm)
		if !typeOk {
			// log.Fatal("bad type imm")
			continue
		}

		if (inst.Op == x86asm.MOV && inst.Opcode>>16 == 0xc785) && (structAddr < 0 || mem.Disp-structAddr == 4) {
			// fmt.Printf("%# v\n", pretty.Formatter(inst))
			binary.BigEndian.PutUint32(data[(partCount*4):], uint32(imm))

			structAddr = mem.Disp
			partCount++

			if partCount == 4 {
				break
			}
		}
	}

	return data
}

func _getPrivData(buf []byte) []byte {
	sec, arch, base := getTextSection(buf)

	_ = base

	raw, err := sec.Data()
	if err != nil {
		log.Fatal(errors.WithStack(err))
	}
	data := raw
	fileSize := len(raw)
	memSize := int(sec.VirtualSize)
	if fileSize > memSize {
		// Ignore section alignment padding.
		data = raw[:memSize]
	}

	code := data
	start := uint64(0)
	end := uint64(len(code))

	instList := list.New()
	for pc := start; pc < end; {
		addr := pc

		inst, err := x86asm.Decode(code[addr:], arch)
		if err != nil {
			pc++
			continue
		}
		size := inst.Len

		instList.PushBack(inst)
		if instList.Len() > 20 {
			instList.Remove(instList.Front())
		}

		if ok := checkSizeAssigment(&inst); ok {
			// fmt.Printf("%# v\n", pretty.Formatter(inst))
			// fmt.Printf("%016x\t%s\n", base+pc, x86asm.IntelSyntax(inst, pc, nil))
			// Iterate through list and print its contents.
			break
		}

		pc += uint64(size)
	}

	if data := extractFromInstList(instList); data != nil {
		return data
	}

	fmt.Println("this is so bad")
	return nil
}

func getDllPath() (string, error) {
	if _, err := os.Stat(streamPath); err == nil {
		return streamPath, nil
	}

	return getDownloadDllPath()
}

func getPrivData() []byte {
	dllPath, err := getDllPath()
	if err != nil {
		log.Fatal(err)
	}

	buf, err := ioutil.ReadFile(dllPath)
	if err != nil {
		log.Fatal(err)
	}

	data := _getPrivData(buf)
	return data
}

func checkValidData(priv []byte) bool {
	sum := crc32.ChecksumIEEE(priv)
	for _, validChecksum := range validChecksums {
		if sum == validChecksum {
			return true
		}
	}
	return false
}
