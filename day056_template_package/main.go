package main

import (
	"html/template"
	"os"
)

// Post 구조체는 템플릿에 전달될 데이터 모델입니다.
type Post struct {
	Title   string
	Content string
}

func main() {
	// 1. 템플릿 정의
	// html/template 패키지를 사용하면 자동으로 HTML 이스케이프 처리가 됩니다.
	// text/template를 사용하면 이스케이프가 발생하지 않습니다.
	const templateString = `
<!DOCTYPE html>
<html>
<body>
	<h1>{{.Title}}</h1>
	<div>{{.Content}}</div>
</body>
</html>`

	// 2. 템플릿 파싱
	// html/template.New()로 새 템플릿 생성 후 Parse() 메서드로 문자열 파싱
	tmpl, err := template.New("post").Parse(templateString)
	if err != nil {
		panic(err)
	}

	// 3. 데이터 생성
	data := Post{
		Title:   "Go 템플릿 학습",
		Content: "<script>alert('XSS 공격')</script>",
	}

	// 4. 템플릿 실행 (Execution)
	// os.Stdout은 템플릿의 출력을 화면으로 보냅니다.
	// Execute() 호출 시 템플릿 엔진이 데이터를 대입하여 바이트열을 생성합니다.
	if err := tmpl.Execute(os.Stdout, data); err != nil {
		panic(err)
	}

	// 결과: Content 필드의 <script> 태그가 HTML 문자로 변환되어 출력됨을 확인할 수 있습니다.
}