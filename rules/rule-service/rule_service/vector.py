import os
import nltk
from fastapi import FastAPI, File, UploadFile, HTTPException
from fastapi.responses import JSONResponse
from pydantic_settings import BaseSettings
import logging
from transformers import AutoModel, AutoTokenizer
import torch
import torch.nn.functional as F
import psycopg2
from psycopg2.extras import execute_values
from typing import List
from nltk.tokenize import sent_tokenize, word_tokenize
import PyPDF2
import docx
from sqlalchemy import create_engine, Column, Integer, String, Text, Float
from sqlalchemy.dialects.postgresql import ARRAY
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker

# Download NLTK data
nltk.download('punkt')
nltk.download('punkt_tab')

# Environment variables and settings
class Settings(BaseSettings):
    DB_HOST: str = "localhost"
    DB_PORT: int = 5432
    DB_NAME: str = "vectordb"
    DB_USER: str = "vectoruser"
    DB_PASS: str = "vectorpass123"
    CHUNK_SIZE: int = 100
    MODEL_PATH: str = "Alibaba-NLP/gte-base-en-v1.5"

    class Config:
        env_file = ".env"


settings = Settings()

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI()

# Initialize model and tokenizer
tokenizer = AutoTokenizer.from_pretrained(settings.MODEL_PATH)
model = AutoModel.from_pretrained(settings.MODEL_PATH, trust_remote_code=True)
# SQLAlchemy setup
DATABASE_URL = f"postgresql://{settings.DB_USER}:{settings.DB_PASS}@{settings.DB_HOST}:{settings.DB_PORT}/{settings.DB_NAME}"
engine = create_engine(DATABASE_URL)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)
Base = declarative_base()

# Define SQLAlchemy model
class DocumentEmbedding(Base):
    __tablename__ = "document_embeddings"

    id = Column(Integer, primary_key=True, index=True)
    filename = Column(String)
    chunk_text = Column(Text)
    embedding = Column(ARRAY(Float))

# Create tables
Base.metadata.create_all(bind=engine)

# Database connection
def get_db():
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()

# Function to create chunks
def create_chunks(text: str, chunk_size: int) -> List[str]:
    sentences = sent_tokenize(text)
    chunks = []
    current_chunk = []
    current_chunk_size = 0

    for sentence in sentences:
        words = word_tokenize(sentence)
        if current_chunk_size + len(words) <= chunk_size:
            current_chunk.extend(words)
            current_chunk_size += len(words)
        else:
            chunks.append(" ".join(current_chunk))
            current_chunk = words
            current_chunk_size = len(words)

    if current_chunk:
        chunks.append(" ".join(current_chunk))

    return chunks


# Function to create embeddings
def create_embeddings(chunks: List[str]):
    try:
        batch_dict = tokenizer(chunks, max_length=8192, padding=True, truncation=True, return_tensors='pt')
        with torch.no_grad():
            outputs = model(**batch_dict)
        embeddings = outputs.last_hidden_state[:, 0]
        embeddings = F.normalize(embeddings, p=2, dim=1)
        return embeddings.tolist()
    except Exception as e:
        logger.error(f"Error creating embeddings: {e}")
        raise HTTPException(status_code=500, detail="Error creating embeddings")


# Function to extract text from various file types
def extract_text(file: UploadFile) -> str:
    try:
        _, file_extension = os.path.splitext(file.filename)

        if file_extension.lower() == '.pdf':
            pdf_reader = PyPDF2.PdfReader(file.file)
            text = ""
            for page in pdf_reader.pages:
                text += page.extract_text()
        elif file_extension.lower() in ['.doc', '.docx']:
            doc = docx.Document(file.file)
            text = "\n".join([para.text for para in doc.paragraphs])
        elif file_extension.lower() in ['.txt', '.md']:
            text = file.file.read().decode('utf-8')
        else:
            raise ValueError(f"Unsupported file type: {file_extension}")

        return text
    except Exception as e:
        logger.error(f"Error extracting text from file: {e}")
        raise HTTPException(status_code=400, detail=f"Error extracting text from file: {str(e)}")


@app.post("/upload")
async def upload_file(file: UploadFile = File(...)):
    try:
        # Extract text from file
        text = extract_text(file)

        # Create chunks
        chunks = create_chunks(text, settings.CHUNK_SIZE)
        logger.info(f"Created {len(chunks)} chunks")

        # Create embeddings
        embeddings = create_embeddings(chunks)
        logger.info(f"Created {len(embeddings)} embeddings")

        # Store in database
        db = next(get_db())

        # Insert data
        for chunk, embedding in zip(chunks, embeddings):
            db_embedding = DocumentEmbedding(
                filename=file.filename,
                chunk_text=chunk,
                embedding=embedding
            )
            db.add(db_embedding)

        db.commit()

        return JSONResponse(content={"message": f"File processed and {len(chunks)} embeddings stored in the database"})

    except Exception as e:
        logger.error(f"Error processing file: {e}")
        raise HTTPException(status_code=500, detail=f"Error processing file: {str(e)}")

if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8001)