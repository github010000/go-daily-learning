package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
)

// Todo: REST API에서 관리할 할 일 데이터 구조
type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// 전역 상태 및 동시성 보호용 락
var (
	todos    []Todo
	mu       sync.Mutex
	nextID   int
)

// listTodos: GET /todos - 등록된 모든 할 일 반환
func listTodos(w http.ResponseWriter, r *http.Request) {
	// 동시성 보호: 읽기 전에 락 획득
	mu.Lock()
	defer mu.Unlock()

	// 응답 헤더 설정 (JSON 형식)
	w.Header().Set("Content-Type", "application/json")
	// 상태 코드 200 OK 설정
	w.WriteHeader(http.StatusOK)
	// 슬라이스를 JSON 인코딩하여 응답 바디로 전달
	json.NewEncoder(w).Encode(todos)
}

// createTodo: POST /todos/create - 새로운 할 일 추가
func createTodo(w http.ResponseWriter, r *http.Request) {
	// 동시성 보호
	mu.Lock()
	defer mu.Unlock()

	var newTodo Todo
	// 요청 본체(Body)의 JSON 데이터를 구조체로 디코딩
	if err := json.NewDecoder(r.Body).Decode(&newTodo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 고유 ID 부여 및 슬라이스에 추가
	nextID++
	newTodo.ID = nextID
	todos = append(todos, newTodo)

	// 응답 설정: 201 Created + 생성된 데이터
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newTodo)
}

// deleteTodo: DELETE /todos/delete?id=ID - ID에 해당하는 할 일 삭제
func deleteTodo(w http.ResponseWriter, r *http.Request) {
	// 동시성 보호
	mu.Lock()
	defer mu.Unlock()

	// URL 쿼리 파라미터에서 ID 추출
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		http.Error(w, "id 파라미터가 유효하지 않습니다.", http.StatusBadRequest)
		return
	}

	// ID가 일치하는 항목을 슬라이스에서 제거
	for i, t := range todos {
		if t.ID == id {
			todos = append(todos[:i], todos[i+1:]...)
			break
		}
	}

	// 204 NoContent: 성공했지만 반환할 바디가 없는 경우
	w.WriteHeader(http.StatusNoContent)
}

func main() {
	// ServeMux 인스턴스 생성 (기본 라우터)
	mux := http.NewServeMux()

	// URL 패턴과 핸들러 함수 매핑
	mux.HandleFunc("/todos", listTodos)
	mux.HandleFunc("/todos/create", createTodo)
	mux.HandleFunc("/todos/delete", deleteTodo)

	// 서버 실행 전 출력 (예제 확인용)
	fmt.Println("🚀 HTTP 서버 시작: http://localhost:8080")
	fmt.Println("📝 GET    /todos           : 전체 조회")
	fmt.Println("📦 POST   /todos/create    : 항목 추가")
	fmt.Println("🗑️  DELETE /todos/delete?id=1 : 항목 삭제")

	// 서버 구동 (블로킹 함수: 에러 발생 전까지 대기)
	log.Fatal(http.ListenAndServe(":8080", mux))
}