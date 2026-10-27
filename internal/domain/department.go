package domain

type Department struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}

type CreateDepartmentRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}

type UpdateDepartmentRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}