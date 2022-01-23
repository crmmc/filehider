package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var disablesha1 bool                       //SHA1校验开关
var enablerename bool                      //乱序文件名格式开关
var onlytest bool                          //仅测试开关
var fileextname string = "mp4"             //输出的加密文件的后缀名
var cfilename string = "_"                 //还原的文件的文件名前缀，防止源文件存在导致覆盖写入
var buffersize int = 4096                  //读入缓存空间大小
var globalmaxthread int = runtime.NumCPU() //线程池中任务线程的数量
var outputpath string = ""
var fileheadle []byte = []byte{
	0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x6D, 0x70, 0x34, 0x32, 0x00, 0x00, 0x00, 0x00,
	0x6D, 0x70, 0x34, 0x32, 0x6D, 0x70, 0x34, 0x31, 0x00, 0x04, 0x38, 0xDC, 0x6D, 0x6F, 0x6F, 0x76,
	0x00, 0x00, 0x00, 0x6C, 0x6D, 0x76, 0x68, 0x64, 0x00, 0x00, 0x00, 0x00, 0xDD, 0x53, 0x54, 0x50,
	0xDD, 0x53, 0x54, 0x5C, 0x00, 0x01, 0x5F, 0x90, 0x01, 0xD5, 0x94, 0x3F, 0xDD, 0x53} //用于校验和伪装的文件头

var gw = sync.WaitGroup{}               //用于进程池阻塞主函数
var maintasklist = make(chan string, 1) //用于向子线程传递参数
var allfailed []string                  //储存失败的文件方便报告
var allfailedml = sync.Mutex{}          //储存失败文件的变量的互斥锁

func main() {
	argv := os.Args   //系统传入的命令行参数
	argc := len(argv) //系统传入的命令行参数个数
	if argc == 1 {
		help(argv[0])
		os.Exit(0)
	}
	//开始检查命令行参数
	for i := 1; i < argc; i++ {
		if argv[i] == "-h" {
			help(argv[0])
			os.Exit(0)
		}
		if argv[i] == "-n" {
			disablesha1 = true //-n开关被开启
			continue
		}
		if argv[i] == "-t" {
			onlytest = true //-t开关被开启
			continue
		}
		if argv[i] == "-s" {
			enablerename = true //-s开关被开启
			//fmt.Println("-s开关被开启")
			continue
		}
		if argv[i] == "-f" {
			cfilename = "" //-f开关被开启
			continue
		}
		if len(argv[i]) > 6 {
			if argv[i][0:6] == "--ext=" {
				fileextname = argv[i][6:] //设定输出后缀名
				fmt.Println("SET EXT: ." + fileextname)
				continue
			}
		}
		if len(argv[i]) > 13 {
			if argv[i][0:13] == "--outputpath=" {
				outputpath = argv[i][13:] //提取输出路径
				if outputpath == "" {
					outputpath = "./"
				} else if outputpath[len(outputpath)-1:] != "/" {
					if outputpath[len(outputpath)-1:] != "\\" {
						if strings.Contains(outputpath, "\\") {
							outputpath = outputpath + "\\"
						} else if strings.Contains(outputpath, "/") {
							outputpath = outputpath + "/"
						}
					}
				}
				if info, err := os.Stat(outputpath); err != nil {
					fmt.Println("Uncorrect folder Path" + outputpath)
					fmt.Println("Reset Output Path to ./")
					outputpath = "./"
				} else if info.IsDir() {
					fmt.Println("Reset Output DIR: " + outputpath)
				}
				continue
			} else {
				if argv[i][0:12] == "--maxthread=" {
					var nerr error
					globalmaxthread, nerr = strconv.Atoi(argv[i][12:]) //读取最大核心数
					if nerr != nil {
						globalmaxthread = runtime.NumCPU()
					}
					fmt.Printf("Reset MAX ThreadNum: %d\n", globalmaxthread)
					continue
				}
			}
		}
		if info, err := os.Stat(argv[i]); err != nil {
			fmt.Println("Unrecongizeable Parameter: " + argv[i])
		} else if info.IsDir() {
			fmt.Printf("Reading files from dir: %s\n", argv[i])
			filelist, err := ioutil.ReadDir(argv[i])
			if err != nil {
				fmt.Printf("Error while reading %s: [%s]\n", argv[i], err.Error())
			}
			for _, f := range filelist {
				maintasklist <- filepath.Join(argv[i], "./", f.Name())
			}
		} else {
			maintasklist <- argv[i]
		}
	}
	//检查命令行参数结束
	//启动线程池
	for threadid := 1; threadid < globalmaxthread; threadid++ {
		// log.Printf("SUB Thread %d start!", threadid)
		go subthread() //线程池内线程先启动等待任务分配
	}
	//启动线程池完成
	time.Sleep(2000)
	gw.Wait() //防止主进程过早退出
	//检查是否有错误记录,有的话集中输出
	if len(allfailed) == 0 {
		fmt.Println("All Tasks Successful!")
		os.Exit(0)
	} else {
		fmt.Println("Error Files List:")
		for i := 0; i < len(allfailed); i++ {
			fmt.Println(allfailed[i])
		}
		fmt.Println("END")
	}
	// 检查错误记录结束
	//主进程结束
}

func subthread() {
	for {
		fileinputs := <-maintasklist
		gw.Add(1)
		ret := process(fileinputs)
		errreport := ""
		if ret != 0 {
			if ret == 3 {
				errreport = "Open Output File For " + fileinputs + " Failed!"
			} else if ret == 2 {
				errreport = "Open Input File For " + fileinputs + " Failed!"
			} else if ret != 0 {
				errreport = fmt.Sprintf("Doing %s Got Different Return: %d", fileinputs, ret)
			}
			allfailedml.Lock()
			allfailed = append(allfailed, errreport)
			allfailedml.Unlock()
		}
		gw.Add(-1)
	}
}

/*
错误返回码定义：
正常：0
输入文件打开失败：2
输出文件打开失败：3
*/

func process(inputfilename string) int {
	inf, inferr := os.Open(inputfilename) //打开输入文件
	if inferr != nil {
		return 2
	}
	var outfn string = "" //输出文件
	fmt.Println("Input: " + inputfilename)
	//分割输入文件的路径和文件名
	var orfn, orfp string //path模块有毛病啊，为什么返回值全是空字符串啊，害的我自己写
	if strings.Contains(inputfilename, "\\") {
		a2 := strings.Split(inputfilename, "\\")
		orfn = a2[(len(a2) - 1)]
		a2 = a2[:(len(a2) - 1)]
		orfp = a2[0] + "\\"
		for i := 1; i < len(a2); i++ {
			orfp = orfp + a2[i] + "\\"
		}
	} else if strings.Contains(inputfilename, "/") {
		a2 := strings.Split(inputfilename, "/")
		orfn = a2[(len(a2) - 1)]
		a2 = a2[:(len(a2) - 1)]
		orfp = a2[0] + "/"
		for i := 1; i < len(a2); i++ {
			orfp = orfp + a2[i] + "/"
		}
	} else {
		orfp = "./"
		orfn = inputfilename
	}
	//读识别用文件头
	readsize1 := len(fileheadle)
	infheadle := make([]byte, readsize1)
	inf.Read(infheadle)
	sha := sha1.New() //sha1检验对象
	shatext := ""     //储存sha1校验值字符串
	//分析是否符合文件头
	if analyze(fileheadle, infheadle) {
		//符合文件头，判断为已被加密的文件
		fmt.Println("Mode: Decode")
		// 读储存的文件名变量大小
		nbl := make([]byte, 4)
		inf.Read(nbl)
		nl := BytesToInt(nbl) //转换为int
		// 读储存的文件名
		rbn := make([]byte, nl)
		inf.Read(rbn)
		rn := bytes2str(encode(rbn)) //转换为字符串
		fmt.Println("FILE:" + rn)
		if disablesha1 {
			fmt.Println("SHA1 Check: Disable")
		} else {
			fmt.Println("SHA1 Check: Enable")
		}
		rn = cfilename + rn
		if outputpath == "" {
			outfn = orfp + rn //组合输出文件路径
		} else {
			outfn = outputpath + rn //组合设定的文件输出路径
		}
		var outf *os.File
		var oerr error
		if !onlytest {
			outf, oerr = os.Create(outfn)
			if oerr != nil {
				return 3
			}
		}
		rbshadata := make([]byte, 20) //储存从文件读到的sha1
		inf.Read(rbshadata)           //读sha1值
		buf := make([]byte, buffersize)
		for { //没有while true，go里这样实现
			count, rerr := inf.Read(buf)
			if rerr == io.EOF {
				break
			}
			debuf := encode(buf[0:count]) //文件内容解密
			if !disablesha1 {
				sha.Write(debuf) //文件内容读入sha1
			}
			if !onlytest {
				outf.Write(debuf) //写入还原的数据到输出文件
			}
		}
		if !onlytest {
			outf.Close() //写入完成，关闭输出文件
		}
		if !disablesha1 {
			rshadata := sha.Sum(nil) //计算sha1
			if analyze(rshadata, rbshadata) {
				fmt.Println("SHA1 Check: Pass")
			} else {
				fmt.Println("SHA1 Check: File may broken")
			}
		} else {
			fmt.Println("SHA1 Check: Please check file sha1 by yourself")
		}
		enablerename = false
		inf.Close()
		if !onlytest {
			fmt.Println("Ouput: " + outfn)
		}
	} else {
		inf.Seek(0, io.SeekStart) //是个未加密的文件，移动文件指针到文件开头
		if outputpath == "" {
			if enablerename {
				fmt.Println("AutoRename Mode ON")
				outfn = fmt.Sprintf("%x", sha1.New().Sum(str2bytes(inputfilename))) + "." + fileextname //利用sha1生成唯一的文件名
			} else {
				outfn = inputfilename + "." + fileextname //生成文件名
			}
		} else {
			if enablerename {
				fmt.Println("AutoRename Mode ON")
				outfn = outputpath + fmt.Sprintf("%x", sha1.New().Sum(str2bytes(inputfilename))) + "." + fileextname //利用sha1生成唯一的文件名
			} else {
				outfn = outputpath + orfn + "." + fileextname //生成文件名
			}
		}
		fmt.Println("Mode: Encode")
		if disablesha1 {
			fmt.Println("SHA1 Check: Enable")
			fmt.Println("SHA1 Check: ignore \"-n\" switch")
		}
		outf, oerr := os.Create(outfn) //若文件存在则覆盖写入，不存在就创建
		if oerr != nil {
			return 3
		}
		outf.Write(fileheadle) //写文件头
		//转换并写入文件名数据长度和文件名数据
		//orfn正常
		a1 := str2bytes(orfn)
		orfnc := orfn + " " //防止到后面输出全是乱码和被编译器优化导致内存指针指向orfn，只能随便拼接一个无用字符
		outf.Write(IntToBytes(len(a1)))
		//到这里orfn变量的内容会被破坏，输出是乱码，估计和这个函数直接的内存操作有关
		outf.Write(encode(a1))
		wshaseek, _ := outf.Seek(0, io.SeekCurrent) //用于储存sha1写入位置的文件指针
		outf.Write(make([]byte, 20))                //先写个空值占位
		buf := make([]byte, buffersize)             //读缓存
		for {
			count, err := inf.Read(buf)
			if err == io.EOF {
				break
			}
			wrb := buf[0:count]
			sha.Write(wrb)          //sha1数据读入
			outf.Write(encode(wrb)) //数据输出到文件
		}
		shadata := sha.Sum(nil)
		outf.Seek(wshaseek, io.SeekStart)    //回到sha1数据的位置
		shatext = fmt.Sprintf("%x", shadata) //sha1数据转换成字符串好展示
		fmt.Println("SHA1 Check: " + shatext)
		outf.Write(shadata) //写sha1值到文件
		outf.Close()
		//产生文件报告
		if enablerename {
			report, lerr := os.Create(outfn + ".report.txt")
			if lerr == nil {
				report.WriteString("原始文件名:" + orfnc + "\n更改后文件名:" + outfn + "\n原始文件sha1:" + shatext)
				report.Close()
			} else {
				fmt.Println("Waring! Report General Failed!")
				report.Close()
			}
		}
		inf.Close()
		fmt.Println("Ouput: " + outfn)
	}
	return 0
}

//帮助
func help(argv0 string) {
	fmt.Println("The FileHider")
	fmt.Println("A simple tool to hide your file")
	fmt.Println("Usage: " + argv0 + " [Option] [File1] [File2] [path1] [path2] [File...] [path...]")
	fmt.Println("Program will automatic encode/decode files")
	fmt.Println("Options:\n\t\"-n\" use for disable sha1 check,but it can only be used when decoding file")
	fmt.Println("\t\"-f\" Remove the \"_\" before the output file name")
	fmt.Println("\t\"-s\" Turn on the auto rename switch,the output file name can cheat mechine prefectly")
	fmt.Println("\t\"--ext=\" can reset the output file suffix (default: mp4)")
	fmt.Println("\t\"--outputpath=\" can set the path for all output files (defalut:[like the input file])")
	fmt.Println("\t\"--maxthread=[int]\" set max threads number for filehider,default equal cpu 's cores number")
	fmt.Println("\t\"-t\" Enable the only test mode, will only test encrypted file data but not write decrypted data to new file")
	fmt.Println("Example:\n\t" + argv0 + " C:/pagefile.sys D:/test.zip E:/files --ext=mov --outputpath=./output/ -n -s")
}

//二进制数据逐字节比较
func analyze(bigbyte, smallbyte []byte) bool {
	for i := 0; i < len(bigbyte); i++ {
		if bigbyte[i] != smallbyte[i] {
			return false
		}
	}
	return true
}

//简单的取反加密函数
func encode(inb []byte) []byte {
	for i := 0; i < len(inb); i++ {
		inb[i] = ^inb[i]
	}
	return inb
}

//整形转换成字节
func IntToBytes(n int) []byte {
	var v2 uint32 = uint32(n)
	var b2 []byte = make([]byte, 4)
	b2[3] = uint8(v2)
	b2[2] = uint8(v2 >> 8)
	b2[1] = uint8(v2 >> 16)
	b2[0] = uint8(v2 >> 24)
	return b2
}

//字节转换成整形
func BytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)
	var x int32
	binary.Read(bytesBuffer, binary.BigEndian, &x)
	return int(x)
}

//高效的字符串转字节组
func str2bytes(s string) []byte {
	x := (*[2]uintptr)(unsafe.Pointer(&s))
	h := [3]uintptr{x[0], x[1], x[1]}
	return *(*[]byte)(unsafe.Pointer(&h))
}

//高效的字节组转字符串
func bytes2str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
