export type ModuleItem = {
  id: string
  title: string
  icon?: string
}

export type Student = {
  id: string
  full_name: string
  student_fid?: string
  sp_id?: string
  person_id?: string
  status?: string
  status_type?: string
  surname?: string
  firstname?: string
  fathersname?: string
  phone?: string
  mobile?: string
  email?: string
  email_uet?: string
  document_id?: string
  document_type?: string
  department?: string
  program_id?: string
  program?: string
  program_specialty?: string
  reg_date?: string
  reg_year?: string
  faculty?: string
  study_level?: string
  nationality?: string
  nat_code?: string
  city?: string
  address?: string
  lastsyncdate?: string
  url: string
}

export type Students = Student[]

export type Employee = {
  id: number | string
  full_name: string
  email?: string
  image?: string
  url: string
}

export type Employees = Employee[]

export type DataResponse = {
  id: string
  title: string
  data: Students | Employees
}

export type SearchResponse = {
  data: DataResponse[]
  modules?: ModuleItem[]
  module?: ModuleItem
  keyword?: string
}

export type SearchFn = (params: { keyword: string }) => Promise<SearchResponse>

export function isStudentsGroup(
  group: DataResponse
): group is DataResponse & { data: Students } {
  return group.id === "students"
}

export function isEmployeesGroup(
  group: DataResponse
): group is DataResponse & { data: Employees } {
  return group.id === "employees"
}

export const emptySearchFn: SearchFn = async () => ({ data: [] })
