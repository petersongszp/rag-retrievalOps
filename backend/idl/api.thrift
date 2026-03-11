include "./user/user.thrift"
include "./resume/resume.thrift"
include "./interview/interview.thrift"
include "./prediction/prediction.thrift"

namespace go api


service UserService extends user.UserService {}
service ResumeService extends resume.ResumeService {}
service InterviewService extends interview.InterviewService {}
service PredictionService extends prediction.PredictionService {}