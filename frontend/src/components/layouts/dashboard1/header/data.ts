export type Notification = {
  id: number
  title: string
  role: string
  desc: string
  avatar: string
  status: string
  unread_message: boolean
  type: string
  date: string
}
export const notifications: Notification[] = [
  // {
  //     id: 1,
  //     title: "Your order is placed",
  //     role: "Frontend Developer",
  //     desc: "Amet minim mollit non deser unt ullamco est sit aliqua.",
  //     avatar: "https://i.pravatar.cc/150?img=1",
  //     status: "online",
  //     unread_message: false,
  //     type: "text",
  //     date: "2 days ago"
  // },
  // {
  //     id: 2,
  //     title: "Congratulations Darlene  🎉",
  //     role: "UI/UX Designer",
  //     desc: "Won the monthly best seller badge",
  //     avatar: "https://i.pravatar.cc/150?img=2",
  //     status: "online",
  //     unread_message: true,
  //     type: "text",
  //     date: "11 am"
  // },
  // {
  //     id: 3,
  //     title: "Joaquina Weisenborn",
  //     role: "Town planner",
  //     desc: "Requesting access permission",
  //     avatar: "https://i.pravatar.cc/150?img=3",
  //     status: "busy",
  //     unread_message: true,
  //     type: "confirm",
  //     date: "12 pm"
  // },
  // {
  //     id: 4,
  //     title: "Brooklyn Simmons",
  //     role: "Data scientist",
  //     desc: "Added you to Top Secret Project group...",
  //     avatar: "https://i.pravatar.cc/150?img=4",
  //     status: "online",
  //     unread_message: true,
  //     type: "text",
  //     date: "1 pm"
  // },
  // {
  //     id: 5,
  //     title: "Margot Henschke",
  //     role: "Dietitian",
  //     desc: "Cake pie jelly jelly beans. Marzipan lemon drops halvah cake. Pudding cookie lemon drops icing",
  //     avatar: "https://i.pravatar.cc/150?img=5",
  //     status: "busy",
  //     unread_message: false,
  //     type: "text",
  //     date: "3 pm"
  // },
  // {
  //     id: 6,
  //     title: "Sal Piggee",
  //     role: "Marketing executive",
  //     desc: "Toffee caramels jelly-o tart gummi bears cake I love ice cream lollipop. Sweet liquorice croissant candy danish dessert icing. Cake macaroon gingerbread toffee sweet.",
  //     avatar: "https://i.pravatar.cc/150?img=6",
  //     status: "online",
  //     unread_message: false,
  //     type: "text",
  //     date: "4 pm"
  // },
  // {
  //     id: 7,
  //     title: "Miguel Guelff",
  //     role: "Special educational needs teacher",
  //     desc: "Biscuit powder oat cake donut brownie ice cream I love soufflé. I love tootsie roll I love powder tootsie roll.",
  //     avatar: "https://i.pravatar.cc/150?img=7",
  //     status: "online",
  //     unread_message: true,
  //     type: "text",
  //     date: "7 pm"
  // },
  // {
  //     id: 8,
  //     title: "Mauro Elenbaas",
  //     role: "Advertising copywriter",
  //     desc: "Bear claw ice cream lollipop gingerbread carrot cake. Brownie gummi bears chocolate muffin croissant jelly I love marzipan wafer.",
  //     avatar: "https://i.pravatar.cc/150?img=8",
  //     status: "away",
  //     unread_message: true,
  //     type: "text",
  //     date: "10 pm"
  // },
  // {
  //     id: 9,
  //     title: "Bridgett Omohundro",
  //     role: "Designer, television/film set",
  //     desc: "Gummies gummi bears I love candy icing apple pie I love marzipan bear claw. I love tart biscuit I love candy canes pudding chupa chups liquorice croissant.",
  //     avatar: "https://i.pravatar.cc/150?img=9",
  //     status: "offline",
  //     unread_message: false,
  //     type: "text",
  //     date: "10 pm"
  // },
  // {
  //     id: 10,
  //     title: "Zenia Jacobs",
  //     role: "Building surveyor",
  //     desc: "Cake pie jelly jelly beans. Marzipan lemon drops halvah cake. Pudding cookie lemon drops icing",
  //     avatar: "https://i.pravatar.cc/150?img=10",
  //     status: "away",
  //     has_notification: false,
  //     type: "text",
  //     date: "10 am"
  // }
]

export type NotificationType = (typeof notifications)[number]
