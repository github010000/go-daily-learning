package main

import (
	"fmt"
	"reflect"
)

// Person: 리플렉션으로 필드와 태그를 읽을 대상 구조체
type Person struct {
	Name string `json:"full_name"`
	Age  int    `json:"age_years"`
	City string `json:"location"`
}

func main() {
	// 1단계: 직렬화할 대상 인스턴스 생성
	p := Person{Name: "홍길동", Age: 30, City: "서울"}

	// 2단계: reflect.TypeOf과 ValueOf로 메타데이터 및 값 획득
	// TypeOf: 타입 정보(Type) 반환, ValueOf: 실제 값(Value) 반환
	t := reflect.TypeOf(p)
	v := reflect.ValueOf(p)

	fmt.Println("=== 리플렉션 기본 정보 ===")
	fmt.Printf("타입(Type): %v\n", t)
	fmt.Printf("종류(Kind): %v\n", t.Kind())
	fmt.Printf("값(Value): %v\n", v)

	// 3단계: 구조체인지 Kind()로 체크 후 필드 순회
	// 리플렉션 사용 전 반드시 Kind() 검사를 통해 타입 안전성 확보
	if t.Kind() != reflect.Struct {
		fmt.Println("지원되지 않는 타입입니다.")
		return
	}

	fmt.Println("\n=== 필드 및 JSON 태그 읽기 ===")
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)       // 필드 타입 메타데이터
		value := v.Field(i)       // 필드 실제 값
		jsonTag := field.Tag.Get("json") // 구조체 태그에서 json 키 추출

		fmt.Printf("필드명: %-5s | JSON 키: %-12s | 값: %-10v\n",
			field.Name, jsonTag, value.Interface())
	}

	// 4단계: 리플렉션을 활용한 간단한 직렬화 시뮬레이션
	fmt.Println("\n=== 간단한 JSON 직렬화 결과 ===")
	var jsonStr string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("json")
		val := v.Field(i).Interface()
		if i > 0 {
			jsonStr += ", "
		}
		jsonStr += fmt.Sprintf(`"%s": %v`, tag, val)
	}
	fmt.Printf("{ %s }\n", jsonStr)

	// 리플렉션 사용 지침: 컴파일러의 타입 검사를 우회하므로 성능 저하 및 유지보수 위험이 있습니다.
	// 프레임워크나 라이브러리 등 필수적인 경우에만 사용해야 합니다.
}