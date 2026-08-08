import AuthorizationLayout from "./components/layout"
import ForgetPassword from "./pages/forget-password"
import Register from "./pages/register"
import SignIn from "./pages/sign-in"

const authorizationRoutes = [
  {
    path: "/",
    element: <AuthorizationLayout />,
    children: [
      { path: "/sign-in", element: <SignIn /> },
      { path: "/register", element: <Register /> },
      { path: "/forgot-password", element: <ForgetPassword /> },
    ],
  },
]

export default authorizationRoutes
