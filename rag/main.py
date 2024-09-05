import json
import os
import nltk
from fastapi import FastAPI, File, UploadFile, HTTPException, Depends, Form
from fastapi.responses import JSONResponse
from langchain.text_splitter import NLTKTextSplitter
from pydantic import BaseModel
import logging
from transformers import AutoModel, AutoTokenizer
import torch
import torch.nn.functional as F
from sqlalchemy import create_engine, Column, Integer, String, Text, Float
from sqlalchemy.dialects.postgresql import ARRAY
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker
import PyPDF2
import docx
from typing import List, Optional
from sentence_transformers import SentenceTransformer
import joblib
from huggingface_hub import hf_hub_download

# Download NLTK data
nltk.download('punkt')
nltk.download('punkt_tab')


# Pydantic model for API input
class APISettings(BaseModel):
    database_url: str
    chunk_size: Optional[int] = None
    chunk_overlap: Optional[int] = None

# Environment variables and settings
class Settings(BaseModel):
    MODEL_PATH: str = "Alibaba-NLP/gte-base-en-v1.5"

    class Config:
        env_file = ".env"


settings = Settings()

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Initialize FastAPI app
app = FastAPI()

# Initialize models and tokenizers
tokenizer = AutoTokenizer.from_pretrained(settings.MODEL_PATH)
model = AutoModel.from_pretrained(settings.MODEL_PATH, trust_remote_code=True)


# PIIDetectorWithML class
class PIIDetectorWithML:
    def __init__(self):
        print("Loading embedding model...")
        embed_model_id = "nomic-ai/nomic-embed-text-v1"
        self.model = SentenceTransformer(model_name_or_path=embed_model_id, trust_remote_code=True)
        print("Loading classifier model...")
        REPO_ID = "Intel/business_safety_logistic_regression_classifier"
        FILENAME = "lr_clf.joblib"
        self.clf = joblib.load(hf_hub_download(repo_id=REPO_ID, filename=FILENAME))
        print("ML detector instantiated successfully!")

    def detect_pii(self, text):
        print("Scanning text with ML detector...")
        embeddings = self.model.encode(text, convert_to_tensor=True).reshape(1, -1).cpu()
        predictions = self.clf.predict(embeddings)
        return True if predictions[0] == 1 else False


# Initialize PIIDetectorWithML
pii_detector = PIIDetectorWithML()

# SQLAlchemy setup
Base = declarative_base()


# Define SQLAlchemy model
class DocumentEmbedding(Base):
    __tablename__ = "document_embeddings"

    id = Column(Integer, primary_key=True, index=True)
    filename = Column(String)
    chunk_text = Column(Text)
    embedding = Column(ARRAY(Float))




# Database connection
def get_db(api_settings: APISettings):
    engine = create_engine(api_settings.database_url)
    SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

    # Create tables
    Base.metadata.create_all(bind=engine)

    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()


# Function to create embeddings
def create_embeddings(texts: List[str]):
    try:
        batch_dict = tokenizer(texts, max_length=8192, padding=True, truncation=True, return_tensors='pt')
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


@app.post("/upload_sensitive_detection")
async def upload_sensitive_detection(file: UploadFile = File(...), api_settings: str = Form(...)):
    try:
        settings = APISettings(**json.loads(api_settings))

        # Extract text from file
        text = extract_text(file)

        # Split the text into sentences
        sentences = nltk.sent_tokenize(text)

        # Detect sensitive data
        sensitive_sentences = [sentence for sentence in sentences if pii_detector.detect_pii(sentence)]

        if sensitive_sentences:
            logger.info(f"Detected {len(sensitive_sentences)} sensitive sentences in the file")

            # Create embeddings only for sensitive sentences
            embeddings = create_embeddings(sensitive_sentences)

            # Store in database
            db = next(get_db(settings))

            # Insert data
            for sentence, embedding in zip(sensitive_sentences, embeddings):
                db_embedding = DocumentEmbedding(
                    filename=file.filename,
                    chunk_text=sentence,
                    embedding=embedding
                )
                db.add(db_embedding)

            db.commit()

            return JSONResponse(content={
                "message": f"File processed. {len(sensitive_sentences)} sensitive sentences detected and stored in the database",
                "sensitive_sentences": sensitive_sentences
            })
        else:
            return JSONResponse(content={"message": "No sensitive data detected in the file"})

    except Exception as e:
        logger.error(f"Error processing file: {e}")
        raise HTTPException(status_code=500, detail=f"Error processing file: {str(e)}")


@app.post("/upload")
async def upload_file(file: UploadFile = File(...), api_settings: str = Form(...)):
    try:
        settings = APISettings(**json.loads(api_settings))

        # Extract text from file
        text = extract_text(file)

        # Create chunks using NLTKTextSplitter
        text_splitter = NLTKTextSplitter(chunk_size=settings.chunk_size, chunk_overlap=settings.chunk_overlap)
        chunks = text_splitter.split_text(text)
        logger.info(f"Created {len(chunks)} chunks")

        # Create embeddings
        embeddings = create_embeddings(chunks)
        logger.info(f"Created {len(embeddings)} embeddings")

        # Store in database
        db = next(get_db(settings))

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
