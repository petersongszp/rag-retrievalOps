#!/usr/bin/env python3
"""
创建测试知识库和文档数据的脚本
"""
import requests
import json
import os
from pathlib import Path

# API 基础地址
API_BASE = "http://localhost:8899/api"

def create_knowledge_base(name, description=""):
    """创建知识库"""
    url = f"{API_BASE}/admin/kb/bases"
    payload = {
        "name": name,
        "description": description
    }
    try:
        response = requests.post(url, json=payload)
        print(f"Create Knowledge Base Status: {response.status_code}")
        if response.status_code == 200 or response.status_code == 201:
            data = response.json()
            print(f"Successfully created KB: {data}")
            return data
        else:
            print(f"Error creating KB: {response.text}")
            return None
    except Exception as e:
        print(f"Exception creating KB: {e}")
        return None

def list_knowledge_bases():
    """列出所有知识库"""
    url = f"{API_BASE}/admin/kb/bases"
    try:
        response = requests.get(url)
        print(f"List Knowledge Bases Status: {response.status_code}")
        if response.status_code == 200:
            data = response.json()
            print(f"Knowledge Bases: {json.dumps(data, indent=2)}")
            return data
        else:
            print(f"Error listing KBs: {response.text}")
            return None
    except Exception as e:
        print(f"Exception listing KBs: {e}")
        return None

def upload_document(kb_id, file_path):
    """上传文档到知识库"""
    url = f"{API_BASE}/admin/kb/documents/upload"
    
    file_name = os.path.basename(file_path)
    
    with open(file_path, 'rb') as f:
        files = {
            'file': (file_name, f)
        }
        data = {
            'kb_id': str(kb_id)
        }
        
        try:
            response = requests.post(url, files=files, data=data)
            print(f"\nUpload Document {file_name} Status: {response.status_code}")
            if response.status_code == 200 or response.status_code == 201:
                data = response.json()
                print(f"Successfully uploaded document: {json.dumps(data, indent=2)}")
                return data
            else:
                print(f"Error uploading document: {response.text}")
                return None
        except Exception as e:
            print(f"Exception uploading document: {e}")
            return None

def list_documents(kb_id):
    """列出知识库中的文档"""
    url = f"{API_BASE}/admin/kb/documents"
    try:
        response = requests.get(url, params={'kb_id': kb_id})
        print(f"\nList Documents Status: {response.status_code}")
        if response.status_code == 200:
            data = response.json()
            print(f"Documents in KB {kb_id}: {json.dumps(data, indent=2)}")
            return data
        else:
            print(f"Error listing documents: {response.text}")
            return None
    except Exception as e:
        print(f"Exception listing documents: {e}")
        return None

def list_jobs():
    """列出所有任务"""
    url = f"{API_BASE}/admin/kb/jobs"
    try:
        response = requests.get(url)
        print(f"\nList Jobs Status: {response.status_code}")
        if response.status_code == 200:
            data = response.json()
            print(f"Jobs: {json.dumps(data, indent=2)}")
            return data
        else:
            print(f"Error listing jobs: {response.text}")
            return None
    except Exception as e:
        print(f"Exception listing jobs: {e}")
        return None

def main():
    print("=== 创建测试知识库和文档 ===\n")
    
    # Step 1: 创建知识库
    print("Step 1: 创建知识库")
    kb = create_knowledge_base(
        name="测试知识库 - 编程语言",
        description="包含 Go、React、Python 等编程语言相关文档"
    )
    
    # 如果创建失败，尝试列出已有的知识库
    if kb is None:
        print("\n尝试列出已有的知识库...")
        kb_list = list_knowledge_bases()
        if kb_list and 'items' in kb_list and len(kb_list['items']) > 0:
            kb = kb_list['items'][0]
            print(f"使用现有知识库: {kb['name']} (ID: {kb['id']})")
    
    if kb is None:
        print("无法创建或获取知识库，脚本终止")
        return
    
    kb_id = kb.get('id') or kb.get('ID')
    print(f"\n使用知识库 ID: {kb_id}")
    
    # Step 2: 上传测试文档
    print("\nStep 2: 上传测试文档")
    
    current_dir = Path(__file__).parent
    test_files = [
        current_dir / "go_introduction.md",
        current_dir / "react_tutorial.md"
    ]
    
    for test_file in test_files:
        if test_file.exists():
            upload_document(kb_id, str(test_file))
        else:
            print(f"\n警告: 文件 {test_file} 不存在")
    
    # Step 3: 列出文档和任务
    print("\nStep 3: 检查数据")
    list_documents(kb_id)
    list_jobs()
    
    print("\n=== 脚本执行完成 ===")

if __name__ == "__main__":
    main()
