import { createConnectTransport } from "@connectrpc/connect-web"
import { createClient } from "@connectrpc/connect"
import { TemplateService } from "@/gen/pkg/rpc/template_pb"

const transport = createConnectTransport({
  baseUrl: window.location.origin,
})

export const rpcClient = createClient(TemplateService, transport)