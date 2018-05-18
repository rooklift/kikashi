import sys, tkinter, tkinter.filedialog

SGF_tuple = ("SGF files", "*.sgf")

root = tkinter.Tk()
root.withdraw()

if "save" in sys.argv:
	file_path = tkinter.filedialog.asksaveasfilename(filetypes = [SGF_tuple], defaultextension=".sgf")
else:
	file_path = tkinter.filedialog.askopenfilename(filetypes = [SGF_tuple])

print(file_path)	# FIXME: Windows Unicode issues
