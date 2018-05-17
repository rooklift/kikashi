import tkinter
from tkinter import filedialog

SGF_tuple = ("SGF files", "*.sgf")

root = tkinter.Tk()
root.withdraw()
file_path = filedialog.askopenfilename(filetypes = [SGF_tuple])
print(file_path, end="")
