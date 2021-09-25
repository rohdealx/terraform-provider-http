data "http" "user" {
  url = "https://jsonplaceholder.typicode.com/users/1"
}

output "user_name" {
  value = jsondecode(data.http.user.response_body).name
}
