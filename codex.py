# Codex 专用命令行工具 - 适配你的第三方接口
from openai import OpenAI
import sys

# 自动配置你的密钥和接口
client = OpenAI(
    api_key="sk-3cADAqxb3Gu9pP3dFehQ9oMmiakBvUVH4d2Pf2fPMbleN4Lf",
    base_url="https://wududu.edu.kg/v1"
)

# 启动界面
print("="*50)
print("Codex CLI 已启动 ✅")
print("输入你的代码需求，输入 q 退出")
print("="*50)

while True:
    try:
        prompt = input("\n请输入需求：")
        if prompt.lower() in ["q", "exit", "quit"]:
            print("退出程序~")
            break
        
        # 调用接口生成代码
        response = client.chat.completions.create(
            model="gpt-3.5-turbo",
            messages=[{"role": "user", "content": prompt}]
        )
        
        # 输出结果
        print("\n【AI 返回结果】")
        print(response.choices[0].message.content)
        
    except Exception as e:
        print(f"\n❌ 错误：{str(e)}")
        print("提示：接口可能临时不稳定，重试即可~")