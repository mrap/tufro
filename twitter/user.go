package twitter

const MaxUserRequests = 5

type UserRequests map[int64]*[]*Request

func (u *UserRequests) Add(req *Request) {
	reqs := u.get(req.User.Id)
	(*reqs) = append((*reqs), req)
	pruneExcess(reqs)
}

func (u *UserRequests) get(userID int64) *[]*Request {
	reqs := (*u)[userID]
	if reqs == nil {
		reqs = &[]*Request{}
		(*u)[userID] = reqs
	}
	return reqs
}

func pruneExcess(reqs *[]*Request) {
	if len(*reqs) > MaxUserRequests {
		over := len(*reqs) - MaxUserRequests
		var removed []*Request
		removed, (*reqs) = (*reqs)[0:over], (*reqs)[over:]
		for _, r := range removed {
			r.Cancel()
		}
	}
}
