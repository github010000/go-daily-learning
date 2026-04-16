package main

import "fmt"

// User 구조체 정의: 이름과 나이를 저장하는 필드를 가짐
type User struct {
	Name string
	Age  int
}

// Car 구조체 정의: 모델명과 연식을 저장하는 필드를 가짐
type Car struct {
	Model string
	Year  int
}

func main() {
	// 1. 구조체 리터럴을 사용한 초기화
	// 필드 이름을 명시하여 생성하는 것이 가장 안전하고 가독성이 좋음
	user1 := User{
		Name: "홍길동",
		Age:  25,
	}

	// 2. 필드 접근 및 수정
	fmt.Println("--- 기본 구조체 사용 ---")
	fmt.Printf("사용자: %s, 나이: %d\n", user1.Name, user1.Age)
	user1.Age = 26 // 필드 값 수정
	fmt.Printf("수정된 나이: %d\n", user1.Age)

	// 3. 포인터를 통한 구조체 접근
	// &연산자를 사용하여 구조체의 주소값을 가져옴
	user2Ptr := &User{Name: "이순신", Age: 40}

	fmt.Println("\n--- 포인터를 통한 접근 ---")
	// Go에서는 포인터를 통해 필드에 접근할 때 (*user2Ptr).Name 대신 
	// user2Ptr.Name 처럼 직접 접근이 가능함 (자동 역참조)
	fmt.Printf("사용자: %s, 나이: %d\n", user2Ptr.Name, user2Ptr.Age)
	
	// 포인터를 통해 값을 변경하면 원본 데이터가 변경됨
	user2Ptr.Age = 41
	fmt.Printf("포인터로 변경된 나이: %d\n", user2Ptr.Age)

	// 4. 익명 구조체 (Anonymous Struct)
    // 이름이 없는 구조체를 정의함과 동시에 즉시 생성
    // 특정 함수 내에서만 잠깐 사용할 일회성 데이터 구조에 적합함 (예: JSON 응답 처리)
	fmt.Println("\n--- 익명 구조체 사용 ---")
	config := struct {
		APIKey string
		Timeout int
	}{
		APIKey:  "ABC-12345",
		Timeout: 30,
		// 주의: 필드 이름을 생략하고 순서대로 넣을 수도 있지만, 
		// 가독성을 위해 필드명을 명시하는 것을 권장함
	}
	fmt.Printf("설정 정보 - APIKey: %s, Timeout: %d초\n", config.APIKey, config.Timeout)

	// 5. 구조체 내에 구조체 포함 (Composition)
	type Vehicle struct {
		Brand string
		Car   Car // Car 구조체를 필드로 포함
	}

	myCar := Vehicle{
		Brand: "현대",
		Car: Car{
			Model: "아반떼",
			Year:  2023,
		},
	}
	fmt.Println("\n--- 구조체 중첩 사용 ---")
	fmt.Printf("자동차 브랜드: %s, 모델: %s, 연식: %d\n", myCar.Brand, myCar.Car.Model, myCar.Car.Year)
}