package feedback

import (
	"encoding/json"
	"fmt"
	"github.com/yanghai23/GoLib/aterr"
	"github.com/yanghai23/GoLib/atfile"
	"github.com/yanghai23/GoLib/athttp"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"time"
	"initialize"
	"utils"
	"status/statusCode"
	"status/statusMsg"
)

func insertProblemData(uid, aid, problem, contact, timeStr, appName, version string) {

	//插入数据
	stmt, err := initialize.Db.Prepare("INSERT INTO ProblemInfo(accountId,vpnId,problem,contactInfo,date,appName,version) VALUES(?,?,?,?,?,?,?)")
	defer stmt.Close()
	aterr.CheckErr(err)
	res, err := stmt.Exec(uid, aid, problem, contact, timeStr, appName, version)
	aterr.CheckErr(err)
	_, err = res.LastInsertId()
	aterr.CheckErr(err)
}
func insertLogData(uid, time, path, fileName string) {
	//插入数据
	stmt, err := initialize.Db.Prepare("INSERT INTO LogInfo(accountId,date,path,fileName) VALUES(?,?,?,?)")
	defer stmt.Close()
	aterr.CheckErr(err)

	res, err := stmt.Exec(uid, time, path, fileName)
	aterr.CheckErr(err)
	_, err = res.LastInsertId()
	aterr.CheckErr(err)
}

func FindProblemContent(w http.ResponseWriter, r *http.Request) {
	vpnId := r.FormValue("vpnId")
	//查询数据
	rows, err := initialize.Db.Query("SELECT vpnId,accountId,problem,contactInfo,date FROM ProblemInfo WHERE vpnId = ?", vpnId)
	defer rows.Close()
	var data []ProblemInfoParam
	aterr.CheckErr(err)
	for rows.Next() {
		plm := ProblemInfoParam{}
		err = rows.Scan(&plm.VpnId, &plm.AccountId, &plm.Problem, &plm.ContactInfo, &plm.Date)
		aterr.CheckErr(err)
		data = append(data, plm)

	}

	utils.RStatus(w, statusCode.SUCCESS, statusMsg.QUERY_SUCCES, data)

}

type QueryBean struct {
	Count int
}

func LastNewContent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	con, _ := ioutil.ReadAll(r.Body) //获取post的数据
	qb := &QueryBean{}
	json.Unmarshal(con, &qb)
	//查询数据
	rows, err := initialize.Db.Query("SELECT _id,vpnId,accountId,problem,contactInfo,date FROM ProblemInfo ORDER BY _id DESC limit ?", qb.Count)
	defer rows.Close()
	var data []ProblemInfoParam
	aterr.CheckErr(err)
	for rows.Next() {
		plm := ProblemInfoParam{}
		err = rows.Scan(&plm.Id, &plm.VpnId, &plm.AccountId, &plm.Problem, &plm.ContactInfo, &plm.Date)
		aterr.CheckErr(err)
		res, _ := json.Marshal(plm)
		fmt.Println(string(res))
		data = append(data, plm)

	}
	utils.RStatus(w, statusCode.SUCCESS, statusMsg.QUERY_SUCCES, "")

}
func GetFile(w http.ResponseWriter, r *http.Request) {
	path := r.FormValue("path")
	fileName := r.FormValue("fileName")
	localPath := path + fileName
	buf, err := ioutil.ReadFile(localPath)
	aterr.CheckErr(err)
	io.WriteString(w, string(buf))

}

func FindLogFile(w http.ResponseWriter, r *http.Request) {
	accountId := r.FormValue("accountId")
	//查询数据
	rows, err := initialize.Db.Query("SELECT accountId,path,fileName,date FROM LogInfo WHERE accountId = ?", accountId)
	defer rows.Close()
	var datas []LogFileParam
	if err != nil {
		panic(statusMsg.CREATE_SQL_OBJ_ERROR)
	}
	for rows.Next() {
		data := LogFileParam{}
		err = rows.Scan(&data.AccountId, &data.Path, &data.FileName, &data.Date)
		aterr.CheckErr(err)
		datas = append(datas, data)
	}
	utils.RStatus(w, statusCode.SUCCESS, statusMsg.QUERY_SUCCES, "")
}

func UploadLog(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Header.Get("content-type"))
	if r.Method == "POST" {
		uid := r.PostFormValue("uId")
		problem := r.PostFormValue("problem")
		contactInfo := r.PostFormValue("contactInfo")
		vpnId := r.PostFormValue("vpnId")
		appName := r.PostFormValue("appName")
		version := r.PostFormValue("version")

		sendMsg2DD(uid, vpnId, problem, contactInfo, appName)

		//将反馈意见存入到数据库，方便后期查询
		currentTime := time.Now().Format("2006-01-02T15:04:05Z07:00")

		insertProblemData(uid, vpnId, problem, contactInfo, currentTime, appName, version)
		////如果有文件，则上传文件,log为上传文件的tag

		mf := r.MultipartForm
		fmt.Print(mf.Value)
		if mf != nil {
			files := mf.File["log"]
			if files != nil { //当上传的有文件时，写入文件
				WriteFile2Local(uid, files)
			}
		}

		utils.RStatus(w, statusCode.SUCCESS, statusMsg.SUBMIT_SUCCES, "")
	}
}

/**
  发送消息到钉钉
*/
func sendMsg2DD(uid, VpnId, Problem, ContactInfo, appName string) {
	feedBackMsg := make(map[string]interface{})
	feedBackMsg["accountId"] = uid
	feedBackMsg["vpnId"] = VpnId
	feedBackMsg["appName"] = appName
	feedBackMsg["反馈内容"] = Problem
	feedBackMsg["联系方式"] = ContactInfo
	feedBackMsg["意见和日志文件链接"] = initialize.BaseConfig.BgUrl + uid
	msg, _ := json.Marshal(feedBackMsg)

	athttp.HttpRequest(utils.SendNotify(initialize.BaseConfig.LogRebootUrl, string(msg)))
}

func WriteFile2Local(uid string, fileHeaders []*multipart.FileHeader) error {

	for _, fileHeader := range fileHeaders {

		f, err := fileHeader.Open()
		if err != nil {
			fmt.Println("err -- ", err)
			return err
		}
		data, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Println("err == ", err)
			return err
		}

		//路径是`yibaLog`+时间+uid组成
		path := "yibaLogFile/" + time.Now().Format("2006-01-02") + "/" + uid + "/"

		os.MkdirAll(path, 0777) //创建文件夹
		fileNameStr := time.Now().Format("2006-01-02-15:04:05") + fileHeader.Filename
		file, err := atfile.CreateFile(path, fileNameStr)

		aterr.CheckErr(err)
		buf := []byte(data)
		fmt.Println("strTime", "strTime = "+path)
		fmt.Println("fileNameStr", path+fileNameStr)
		//fmt.Println("buf", "content = "+string(buf))
		file.Write(buf)
		defer file.Close()
		//将日子文件对应关系，存入到数据库，方便查找日志和用户的关系
		insertLogData(uid, time.Now().Format("2006-01-02"), path, fileNameStr)

	}
	return nil
}
