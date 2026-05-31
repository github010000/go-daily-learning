# Day 57: database/sql 기초

## 개념 설명

Go 표준 라이브러리인 `database/sql`은 데이터베이스 추상화 인터페이스를 제공합니다. 이 패키지는 특정 DBMS(데이터베이스 관리 시스템)에 종속되지 않는 공통 API를 정의하며, 실제 연결과 쿼리 실행은 각 DBMS에 맞는 드라이버 패키지(`github.com/mattn/go-sqlite3`, `github.com/go-sql-driver/mysql` 등)가 구현합니다. 이를 통해 개발자는 코드 변경 없이 드라이버만 교체하면 다른 데이터베이스로 마이그레이션할 수 있습니다.

데이터베이스와의 상호작용은 크게 `Exec`, `Query`, `QueryRow` 세 가지 방식으로 나뉩니다. `Exec`은 테이블 생성이나 데이터 삽입/수정/삭제처럼 결과 집합이 필요 없는 DML/DDL 문에 사용하며, `Query`는 복수의 행과 열을 반환하는 SELECT문에 사용됩니다. 단일 행의 특정 컬럼만 조회해야 할 경우 `QueryRow`를 사용하면 코드 가독성과 안전성을 높일 수 있습니다. 모든 쿼리 실행 후 반환된 값은 `Scan` 함수를 통해 Go의 기본 자료형 변수로 안전하게 복사됩니다.

또한 `database/sql`은 Prepared Statement와 트랜잭션을 공식 지원합니다. Prepared Statement는 쿼리 템플릿을 미리 컴파일하여 바인딩 변수를 교체하는 방식으로, 재실행 시 성능을 개선하고 SQL 인젝션 공격을 방어합니다. 트랜잭션(`tx`)은 여러 데이터베이스 작업을 하나의 논리적 단위로 묶어, 어떤 한 단계라도 실패할 경우 전체 작업을 취소(`Rollback`)하거나 성공 시 영구 저장(`Commit`)할 수 있게 합니다. 이는 데이터의 무결성을 유지하는 핵심 기법입니다.

## 코드 설명

- **DB 연결 및 초기화**: `sql.Open`으로 드라이버 이름과 DSN(Data Source Name)을 전달하여 연결 객체를 생성합니다. 실제 연결은 `db.Ping()` 호출 시 검증됩니다. `defer db.Close()`로 리소스 누수를 방지합니다.
- **Exec을 통한 DDL/DML 실행**: `db.Exec()`은 SQL문과 바인딩 인자를 받아 실행합니다. 테이블 생성 및 INSERT문 수행 후 `LastInsertId()`와 영향 받은 행 수를 확인할 수 있습니다.
- **Query & Scan을 통한 데이터 조회**: `db.Query()`로 결과 집합(`*sql.Rows`)을 받습니다. `rows.Next()`로 행을 하나씩 순회하며 `rows.Scan()`으로 컬럼 값을 구조체 또는 변수에 매핑합니다. 루프 종료 후 `rows.Err()`로 순회 중 에러를 반드시 확인합니다.
- **QueryRow & Prepared Statement**: `db.QueryRow()`는 단일 결과만 기대할 때 사용하며, 에러가 없으면 바로 `Scan`합니다. `db.Prepare()`로 미리 컴파일된 준비문을 생성하여 `Exec`에 전달하면 인젝션 방지 및 성능 최적화 효과를 얻습니다.
- **트랜잭션 처리**: `db.Begin()`으로 트랜잭션 객체(`*sql.Tx`)를 얻습니다. 해당 객체로 실행된 모든 쿼리는 커밋 전까지 외부에서 볼 수 없습니다. 성공 시 `tx.Commit()`, 실패 시 `tx.Rollback()`을 호출하여 일관성을 유지합니다.

## 핵심 포인트

- `sql.Open`은 지연 연결이므로 반드시 `db.Ping()`이나 첫 쿼리 실행으로 연결 상태를 검증해야 합니다.
- `rows.Next()`를 사용한 순회 루프 종료 후 `rows.Err()` 체크를 생략하면 버그를 놓칠 수 있으므로 반드시 포함하세요.
- Prepared Statement는 `?` 플레이스홀더를 사용하며, 드라이버마다 파라미터 구문(`?`, `?1`, `$1`)이 다를 수 있으므로 공식 매뉴얼을 참조하세요.
- 트랜잭션 내부에서 `tx` 객체로 쿼리를 실행해야 하며, `db.Exec()`을 사용하면 트랜잭션에 포함되지 않고 즉시 커밋될 수 있습니다.
- 리소스 관리(`defer db.Close()`, `defer rows.Close()`, `defer stmt.Close()`)는 Go의 데이터베이스 프로그래밍에서 필수 관례입니다.

## 참고 링크

- https://pkg.go.dev/database/sql
- https://pkg.go.dev/github.com/mattn/go-sqlite3