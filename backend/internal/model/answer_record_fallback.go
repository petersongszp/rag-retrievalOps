package model

// MergeAnswerRecordsWithDialogueFallback 当答题报告里没有可用的问答对（或 LLM 解析失败）时，
// 使用 interview_dialogues 表中的原始问答填充，供前端「答题全记录复盘」展示。
func MergeAnswerRecordsWithDialogueFallback(
	userID uint,
	reportID uint64,
	records []*AnswerRecordItem,
) []*AnswerRecordItem {
	if answerRecordsHaveMessages(records) {
		return records
	}
	fallback := buildAnswerRecordsFromInterviewDialogues(userID, reportID)
	if len(fallback) > 0 {
		return fallback
	}
	return records
}

func answerRecordsHaveMessages(records []*AnswerRecordItem) bool {
	for _, r := range records {
		if r != nil && len(r.Message) > 0 {
			return true
		}
	}
	return false
}

func buildAnswerRecordsFromInterviewDialogues(userID uint, reportID uint64) []*AnswerRecordItem {
	dialoguesPtr, err := InterviewDialogueDao.GetInterviewDialoguesByUserIdAndRecordId(userID, reportID)
	if err != nil || dialoguesPtr == nil {
		return nil
	}
	dialogues := *dialoguesPtr
	if len(dialogues) == 0 {
		return nil
	}
	msgs := make([]*AnswerRecordMsg, 0, len(dialogues))
	for i, d := range dialogues {
		msgs = append(msgs, &AnswerRecordMsg{
			Order:    int32(i + 1),
			Question: d.Question,
			Answer:   d.Answer,
		})
	}
	return []*AnswerRecordItem{
		{
			Order:   1,
			Content: "面试问答记录",
			Message: msgs,
		},
	}
}
