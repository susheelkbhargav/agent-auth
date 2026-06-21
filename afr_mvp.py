import chromadb
# pyrefly: ignore [missing-import]
from sentence_transformers import SentenceTransformer
import requests

# ---------------------------------------------------------
# 1. INITIALIZATION
# ---------------------------------------------------------
print("Loading embedding model (this takes a few seconds the first time)...")
# This is a lightweight model that runs beautifully on M1 CPUs
embedder = SentenceTransformer('all-MiniLM-L6-v2')

import os

# Initialize a local vector database using an absolute path
db_path = os.path.abspath("./afr_local_db")
client = chromadb.PersistentClient(path=db_path)

collection = client.get_or_create_collection(name="enterprise_docs")

# Clean start for testing (clear old data from the collection)
try:
    existing_data = collection.get()
    if existing_data and existing_data.get('ids'):
        collection.delete(ids=existing_data['ids'])
except Exception as e:
    pass

# ---------------------------------------------------------
# 2. DATA PREPARATION (Tree-Based Chunking & Metadata)
# ---------------------------------------------------------
from langchain_text_splitters import RecursiveCharacterTextSplitter
from datasets import load_dataset

print("Downloading medical dataset (fetching 50 records for MVP)...")
dataset = load_dataset("medalpaca/medical_meadow_wikidoc", split="train[:50]")

text_splitter = RecursiveCharacterTextSplitter(
    chunk_size=400, 
    chunk_overlap=50,
    separators=["\n\n", "\n", ".", " "]
)

documents = []
metadatas = []
ids = []

print("Chunking documents and injecting clean, non-overlapping roles...")
for i, row in enumerate(dataset):
    raw_text = row['input'] + " " + row['output']
    raw_text_lower = raw_text.lower()
    
    # Clean classification to prevent "doctor" role starvation
    # We prioritize financial terms first to ensure the billing admin gets data
    if "billing" in raw_text_lower or "insurance" in raw_text_lower or "cost" in raw_text_lower:
        assigned_role = "billing_admin"
    elif "surgery" in raw_text_lower or "surgical" in raw_text_lower or "transplant" in raw_text_lower:
        assigned_role = "doctor"
    else:
        assigned_role = "general_staff"
        
    chunks = text_splitter.split_text(raw_text)
    for chunk_index, chunk in enumerate(chunks):
        documents.append(chunk)
        metadatas.append({
            "allowed_role": assigned_role, 
            "parent_doc_id": f"doc_{i}"
        })
        ids.append(f"doc_{i}_chunk_{chunk_index}")

print(f"Generated {len(documents)} secured chunks. Embedding now...")
embeddings = embedder.encode(documents).tolist()
collection.add(
    embeddings=embeddings,
    documents=documents,
    metadatas=metadatas,
    ids=ids
)
print("Data embedded and stored securely.\n")

# ---------------------------------------------------------
# 3. THE LLM GENERATION FUNCTION (Ollama)
# ---------------------------------------------------------
def generate_with_phi4(context, user_query):
    """Sends the authorized context and query to your local Phi-4 Mini."""
    prompt = f"Context: {context}\n\nQuestion: {user_query}\nAnswer concisely based ONLY on the context."

    response = requests.post(
        "http://localhost:11434/api/generate",
        json={"model": "phi4-mini", "prompt": prompt, "stream": False}
    )
    return response.json().get("response", "Error generating response.")


# ---------------------------------------------------------
# 4. THE DETERMINISTIC GATEWAY & ROUTER
# ---------------------------------------------------------
def secure_retrieval_agent(query, current_user_role):
    print(f"\n--- New Request Received ---")
    print(f"Query: '{query}'")
    print(f"Verified User Identity: [{current_user_role}]")
    
    query_vector = embedder.encode(query).tolist()
    security_filter = {"allowed_role": current_user_role}
    
    # Execute semantic search with pre-filtering
    results = collection.query(
        query_embeddings=[query_vector],
        n_results=2,
        where=security_filter
    )
    
    # Flatten the result array from ChromaDB
    authorized_docs = results['documents'][0] if results and results.get('documents') else []
    
    # Calculate exact chunk and character counts
    chunk_count = len(authorized_docs)
    char_count = sum(len(doc) for doc in authorized_docs)
    
    # SMART ROUTING
    if chunk_count == 0:
        print("-> Database returned 0 chunks. Deterministic denial triggered.")
        print("-> 0 LLM tokens consumed.")
        return "System Refusal: You do not have authorization to access this information, or it does not exist."
    
    else:
        print(f"-> Database returned {chunk_count} authorized chunks ({char_count} characters of context).")
        print("-> Routing to local Phi-4-Mini...")
        context_string = " ".join(authorized_docs)
        llm_answer = generate_with_phi4(context_string, query)
        return llm_answer

# ---------------------------------------------------------
# 5. EXECUTE THE MVP CONTRAST EVALUATION
# ---------------------------------------------------------
print("\n" + "="*50)
print("STARTING FULL ROLE-BASED ACCESS EVALUATION")
print("="*50)

# Define the queries that target specific metadata domains
clinical_query = "What are the details regarding surgery and surgical treatments?"
financial_query = "What are the insurance and billing protocols?"
general_query = "What are the general guidelines or definitions provided in the text?"

# Map each role to a query they SHOULD be able to answer, and one they SHOULD NOT
roles_to_test = {
    "doctor": {
        "authorized_query": clinical_query,
        "unauthorized_query": financial_query
    },
    "billing_admin": {
        "authorized_query": financial_query,
        "unauthorized_query": clinical_query
    },
    "general_staff": {
        "authorized_query": general_query,
        "unauthorized_query": clinical_query
    }
}

for role, queries in roles_to_test.items():
    print(f"\n\n{'='*50}")
    print(f"TESTING ROLE: {role.upper()}")
    print(f"{'='*50}")

    # Test Authorized Access (Should route to Phi-4 and answer)
    print(f"\n")
    auth_response = secure_retrieval_agent(queries["authorized_query"], current_user_role=role)
    print(f"\nFinal Agent Output:\n{auth_response}")

    # Test Unauthorized Access (Should be blocked at the database level)
    print(f"\n")
    unauth_response = secure_retrieval_agent(queries["unauthorized_query"], current_user_role=role)
    print(f"\nFinal Agent Output:\n{unauth_response}")
