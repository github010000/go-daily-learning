package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	// SQLite3 드라이버 import
	// 실행 전 `go get github.com/mattn/go-sqlite3` 필요 (CGo 환경)
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// 기존 데이터 파일 제거 (재실행 시 초기화)
	os.Remove("mydb.db")

	// 1. DB 연결 (sql.Open은 실제 연결을 생성하지 않고 연결 풀을 준비함)
	db, err := sql.Open("sqlite3", "mydb.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 연결 테스트 및 실제 네트워크/파일 연결 확인
	if err := db.Ping(); err != nil {
		log.Fatal("DB 연결 실패:", err)
	}
	fmt.Println("✅ DB 연결 성공")

	// 2. Exec을 이용한 DDL 및 DML 실행
	// CREATE TABLE: 테이블 존재 여부 확인 후 생성
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT, age INTEGER)")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 테이블 생성 완료")

	// INSERT: 데이터 삽입 및 영향 받은 행/마지막 ID 확인
	res, err := db.Exec("INSERT INTO users (name, age) VALUES (?, ?)", "철수", 25)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ 데이터 삽입 완료 (LastInsertId:", res.LastInsertId(), ")")

	// 3. Query를 이용한 전체 조회 및 Scan
	rows, err := db.Query("SELECT id, name, age FROM users")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("🔍 전체 조회 결과:")
	for rows.Next() {
		var id int
		var name string
		var age int
		// Scan: 결과 집합의 컬럼 순서대로 변수에 값 복사
		if err := rows.Scan(&id, &name, &age); err != nil {
			log.Fatal(err)
		}
		fmt.Printf(" - ID: %d, 이름: %s, 나이: %d\n", id, name, age)
	}
	// Next() 루프 종료 후 에러 확인 필수
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	// 4. QueryRow를 이용한 단일 행 조회
	var targetAge int
	err = db.QueryRow("SELECT age FROM users WHERE name = ?", "철수").Scan(&targetAge)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("🔍 철수의 나이 조회: %d세\n", targetAge)

	// 5. Prepared Statement (재사용 가능한 템플릿 쿼리)
	// SQL 인젝션 방지 및 반복 실행 시 성능 최적화에 유용
	stmt, err := db.Prepare("UPDATE users SET age = ? WHERE name = ?")
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(30, "철수")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✅ Prepared Statement로 데이터 수정 완료")

	// 6. 트랜잭션 (BEGIN, COMMIT, ROLLBACK)
	// 여러 쿼리를 원자성(Atomic)으로 처리하거나 실패 시 전체 취소 가능
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tx.Exec("INSERT INTO users (name, age) VALUES (?, ?)", "영희", 28)
	if err != nil {
		tx.Rollback()
		log.Fatal(err)
	}
	// 명시적 커밋을 호출해야 데이터베이스에 영구 저장됨
	if err = tx.Commit(); err != nil {
		tx.Rollback()
		log.Fatal("트랜잭션 커밋 실패:", err)
	}
	fmt.Println("✅ 트랜잭션 COMMIT 성공")

	// 실패 시 롤백 구조 안내
	fmt.Println("\n📝 tx.Rollback()을 호출하면 BEGIN 이후의 모든 작업이 원상 복구됩니다.")
	fmt.Println("🎉 database/sql 기초 예제 완료")
}