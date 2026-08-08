import {
  SOFTWARES_FETCH_KEY,
  SOFTWARES_LIST_BASE,
  SOFTWARES_INSTALL_BASE,
  enqueueSoftwareActions,
  getSoftwareQueue,
  getSoftwaresList,
  controlSoftwareService,
  type SoftwareListItem,
  type SoftwareQueueAction,
  type SoftwareQueueItem,
  type SoftwareQueueSnapshot,
  type SoftwareEnqueueResponse,
  type SoftwareServiceAction,
} from "../../list/api"

export {
  SOFTWARES_FETCH_KEY,
  SOFTWARES_LIST_BASE,
  SOFTWARES_INSTALL_BASE,
  enqueueSoftwareActions,
  getSoftwareQueue,
  getSoftwaresList,
  controlSoftwareService,
}

export type {
  SoftwareListItem,
  SoftwareQueueAction,
  SoftwareQueueItem,
  SoftwareQueueSnapshot,
  SoftwareEnqueueResponse,
  SoftwareServiceAction,
}

export const INSTALLED_SOFTWARES_FETCH_KEY = "softwares-installed"
