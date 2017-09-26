package twitter

const MaxUserRequests = 5

type UserRequests map[int64][]*Request

func (u UserRequests) Add(req *Request) {
	userID := req.User.Id
	reqs := u.get(userID)
	reqs = append(reqs, req)
	u[userID] = pruneExcess(reqs)
}

func (u UserRequests) get(userID int64) []*Request {
	reqs, ok := u[userID]
	if !ok {
		reqs = []*Request{}
		u[userID] = reqs
	}

	return reqs
}

func pruneExcess(reqs []*Request) []*Request {
	if len(reqs) <= MaxUserRequests {
		return reqs
	}

	over := len(reqs) - MaxUserRequests
	var removed []*Request
	removed, reqs = reqs[0:over], reqs[over:]
	for _, r := range removed {
		r.Cancel()
	}
	return reqs
}
