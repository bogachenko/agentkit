package runtime

type SemanticRouteAction string

const (
	RouteActionPublishDirectAnswer SemanticRouteAction = "PUBLISH_DIRECT_ANSWER"
	RouteActionInitRunState        SemanticRouteAction = "INIT_RUN_STATE"
	RouteActionPublishAskUser      SemanticRouteAction = "PUBLISH_ASK_USER"
	RouteActionPublishReject       SemanticRouteAction = "PUBLISH_REJECT"
)

type SemanticRouteDecision struct {
	Action                         SemanticRouteAction
	UserMessage                    string
	AllowFinalWithoutFreshEvidence bool
}

func DecideSemanticRoute(output ClassifierOutput) SemanticRouteDecision {
	switch output.Route {
	case RouteDirectAnswer:
		return SemanticRouteDecision{Action: RouteActionPublishDirectAnswer, UserMessage: output.UserMessage}
	case RouteExecuteTask:
		return SemanticRouteDecision{Action: RouteActionInitRunState}
	case RouteAnswerFromContext:
		return SemanticRouteDecision{Action: RouteActionInitRunState, AllowFinalWithoutFreshEvidence: true}
	case RouteAskUser:
		return SemanticRouteDecision{Action: RouteActionPublishAskUser, UserMessage: output.UserMessage}
	case RouteRejectUnsupported:
		return SemanticRouteDecision{Action: RouteActionPublishReject, UserMessage: output.UserMessage}
	default:
		return SemanticRouteDecision{Action: RouteActionPublishReject, UserMessage: "Unsupported request route."}
	}
}
