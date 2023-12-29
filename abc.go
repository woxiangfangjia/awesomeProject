package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

type onlineDate struct {
	Id         int    `json:"id"`
	DeviceType int    `json:"deviceType"`
	Username   string `json:"userName"`
	connection *websocket.Conn
} //system

var onlineUsers = make(map[int]onlineDate) //在线id

//func getUserNameById(userId int) (string, error) {
//	var username string
//	query := "SELECT username FROM UserBasicData WHERE userId = ?"
//	err := db.QueryRow(query, userId).Scan(&username)
//
//	switch {
//	case errors.Is(err, sql.ErrNoRows):
//		return "", fmt.Errorf("用户不存在")
//	case err != nil:
//		return "", err
//	default:
//		return username, nil
//	}
//}
//}

var upgrade = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
} //system

type Message struct {
	Command string      `json:"command"`
	Content interface{} `json:"content"`
} //system

func sendJSON(conn *websocket.Conn, message interface{}) {
	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Println("JSON marshal error:", err)
		return
	}

	err = conn.WriteMessage(websocket.TextMessage, jsonData)
	if err != nil {
		log.Println("Write message error:", err)
		return
	}
}

func wsProcessor(w http.ResponseWriter, r *http.Request) {
	upgrade.CheckOrigin = func(r *http.Request) bool { return true }
	ws, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Client Connected to Endpoint...")
	reader(ws)
} //system

func sendMessageFromDB(recipient int) error {
	rows, err := db.Query("SELECT id, sender, recipient, content, timestamp FROM messagedb WHERE recipient = ?", recipient)
	if err != nil {
		return err
	}
	type Message struct {
		ID        int    `json:"id"`
		Sender    int    `json:"sender"`
		Recipient int    `json:"recipient"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
	}
	// 遍历查询结果并发送消息到 WebSocket 连接
	for rows.Next() {
		var message Message
		err := rows.Scan(&message.ID, &message.Sender, &message.Recipient, &message.Content, &message.Timestamp)
		if err != nil {
			return err
		}

		// 发送消息到 WebSocket 连接
		sendJSON(onlineUsers[recipient].connection, message)

		// 删除数据库中的相应行
		err = deleteMessageFromDB(db, message.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteMessageFromDB(db *sql.DB, messageID int) error {
	// 执行删除操作
	_, err := db.Exec("DELETE FROM messagedb WHERE id = ?", messageID)
	return err
}

func reader(conn *websocket.Conn) {
	thisUser := struct {
		userId     int
		userName   string
		deviceType int
		loginState bool
		connection *websocket.Conn
	}{
		connection: conn,
	}
	//init
	thisUser.loginState = false
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			fmt.Println(err)
			return
		}

		var msg Message
		err = json.Unmarshal(p, &msg)
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Printf("Received: %v\n", msg)
		params := msg.Content.(map[string]interface{})
		if thisUser.loginState == false {
			switch msg.Command {
			case "login":
				userID, ok := params["userId"].(float64)
				if !ok {
					sendJSON(thisUser.connection, "invalid params")
					continue
				}
				thisUser.userId = int(userID)

				deviceType, ok := params["deviceType"].(float64)
				if !ok {
					sendJSON(thisUser.connection, "invalid params")
					continue
				}
				thisUser.deviceType = int(deviceType)

				password, ok := params["password"].(string)
				if !ok {
					sendJSON(thisUser.connection, "invalid params")
					continue
				}
				//thisUser.password = password

				var storedPassword string
				query := "SELECT userPassword,userName FROM UserBasicData WHERE userID=?"
				err := db.QueryRow(query, thisUser.userId).Scan(&storedPassword, &thisUser.userName)
				switch {
				case errors.Is(err, sql.ErrNoRows):
					fmt.Println("用户不存在")
				case err != nil:
					fmt.Println("查询错误:", err)
				default:
					// 检查密码是否匹配
					if storedPassword == password {
						fmt.Println("登录成功")
						thisUser.loginState = true
						onlineUsers[thisUser.userId] = onlineDate{thisUser.userId, thisUser.deviceType, thisUser.userName, thisUser.connection}
						err := sendMessageFromDB(thisUser.userId)
						if err != nil {
							fmt.Println("从缓存回发错误：", err)
							return
						}
					} else {
						fmt.Println("密码不匹配")
					}
				}
			case "register":
				userName := params["userName"].(string)
				deviceType := int(params["deviceType"].(float64))
				userPassword := params["userPassword"].(string)
				// 使用userName和deviceType
				// 固定密码

				// 插入记录
				result, err := db.Exec("INSERT INTO UserBasicData (userName, deviceType, userPassword) VALUES (?, ?, ?)", userName, deviceType, userPassword)
				if err != nil {
					fmt.Println("插入记录失败:", err)
					return
				}

				// 获取插入的递增ID
				id, _ := result.LastInsertId()

				fmt.Printf("用户 %s 插入成功，ID：%d\n", userName, id)
				//fmt.Println("in registerProgram")
			default:
				fmt.Println("Permission denied:", msg.Command)
			}
		} else {
			switch msg.Command {
			case "logout":
				delete(onlineUsers, thisUser.userId)
			//case "getOnlineUser":
			//	for key := range onlineUsers {
			//		//fmt.Printf("Key: %d, Value: %+v\n", key, value)
			//		message := MessageBack{Content: onlineUsers[key]}
			//		sendJSON(conn, message)
			//	}
			case "sendMessage":
				var message = struct {
					senderName string
					senderId   int
					content    interface{}
					time       time.Time
				}{}
				message.content = params["content"]
				message.senderId = thisUser.userId
				message.senderName = thisUser.userName
				message.time = time.Now()

				_, ok := onlineUsers[int(params["recipient"].(float64))]
				if !ok {
					_, err := db.Exec("INSERT INTO messagedb (sender, recipient, content, timestamp) VALUES (?, ?, ?, ?)", message.senderId, int(params["recipient"].(float64)), message.content, time.Now())
					if err != nil {
						// 处理错误
						fmt.Println("Error inserting data:", err)
						return
					}
				} else {
					sendJSON(onlineUsers[int(params["recipient"].(float64))].connection, message)
				}

			default:
				fmt.Println("Unknown command:", msg.Command)
			}
		}
		fmt.Println(messageType)
		//if err := conn.WriteMessage(messageType, p); err != nil {
		//	fmt.Println(err)
		//	return
		//}
	}
}

func setupRoutes() {
	http.HandleFunc("/ws", wsProcessor)
} //system

var db *sql.DB

func init() {
	// 初始化数据库连接
	var err error
	db, err = sql.Open("mysql", "root:root@tcp(localhost:3306)/skyflysyncdb")
	if err != nil {
		fmt.Println("数据库连接失败:", err)
		return
	}
} //system

func main() {

	fmt.Println("Go WebSocket")

	//fmt.Println(time.DateTime)
	//fmt.Println(time.Now())

	setupRoutes()
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Printf("http.ListenAndServe: %v", err)
	}

	defer func(db *sql.DB) {
		err := db.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(db) //system
}
