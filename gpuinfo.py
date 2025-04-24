import json

class gpu_Info:
    def __init__(self, name, url):
        self.name = name
        self.url = url
    
    def to_dict(self):
        return {"name": self.name, "url": self.url}
    
    @classmethod
    def from_dict(cls, data):
        return cls(data["name"], data["url"])
