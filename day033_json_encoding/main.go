package main

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// User 구조체는 JSON 직렬화/역직렬화의 대상이 됩니다.
// json 태그를 사용하여 Go 필드명과 JSON 키를 매핑합니다.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Age   int    `json:"age"`
}

func main() {
	// 1단계: Marshal을 사용하여 구조체를 JSON 바이트로 변환합니다.
	fmt.Println("=== 1. Marshal (구조체 -> JSON) ===")
	u := User{ID: 1, Name: "김철수", Email: "chulsoo@example.com", Age: 30}
	data, err := json.Marshal(u)
	if err != nil {
		fmt.Println("Marshal 실패:", err)
		return
	}
	fmt.Printf("변환된 JSON: %s\n\n", string(data))

	// 2단계: Unmarshal을 사용하여 JSON 바이트를 구조체 필드에 매핑합니다.
	fmt.Println("=== 2. Unmarshal (JSON -> 구조체) ===")
	var restored User
	err = json.Unmarshal(data, &restored)
	if err != nil {
		fmt.Println("Unmarshal 실패:", err)
		return
	}
	fmt.Printf("복원된 데이터: ID=%d, Name=%s, Age=%d\n\n", restored.ID, restored.Name, restored.Age)

	// 3단계: Encoder를 사용하여 io.Writer(버퍼)에 JSON 스트림을 순차적으로 씁니다.
	fmt.Println("=== 3. Encoder & Decoder (스트림 처리) ===")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	err = enc.Encode(User{ID: 2, Name: "이영희", Age: 25})
	if err != nil {
		fmt.Println("Encoder 실패:", err)
		return
	}
	fmt.Printf("스트림에 기록된 JSON: %s\n", buf.String())

	// 4단계: Decoder를 사용하여 io.Reader(버퍼)에서 JSON을 다시 읽습니다.
	dec := json.NewDecoder(&buf)
	var decoded User
	err = dec.Decode(&decoded)
	if err != nil {
		fmt.Println("Decoder 실패:", err)
		return
	}
	fmt.Printf("스트림에서 복원된 데이터: %+v\n", decoded)
}