import AuthorizationLayout from "./components/layout"
import ForgetPassword from "./pages/forget-password"
import SignIn from "./pages/sign-in"

const authorizationRoutes = [
  {
    path: "/",
    element: <AuthorizationLayout />,
    children: [
      { path: "/sign-in", element: <SignIn /> },
      { path: "/forgot-password", element: <ForgetPassword /> },
    ],
  },
]

export default authorizationRoutes
